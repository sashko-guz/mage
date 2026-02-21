package parser

import (
	"fmt"
	"strings"

	"github.com/sashko-guz/mage/internal/operations"
)

var (
	// Global operation registry - initialized once at startup via Init
	operationRegistry *operations.Registry
)

// Init initializes the parser with resize dimension limits from config.
// Must be called once at startup before any ParseURL calls.
func Init(maxWidth, maxHeight, maxResolution int) {
	operationRegistry = operations.NewRegistry(maxWidth, maxHeight, maxResolution)
}

// ParseURL parses a URL path and returns a Request with parsed operations
//
// URL Format:
//   - With signature: /thumbs/{signature}/{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
//   - Without signature: /thumbs/{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
//
// Examples:
//   - /thumbs/200x350/filters:format(webp);quality(90)/image.jpg
//   - /thumbs/abc123/200x300/filters:crop(10,10,500,500);fit(cover)/image.jpg
//   - /thumbs/200x350/path/to/source.jpg/as/preview.avif
//
// Operation Rules:
//   - Only ONE operation of each type is allowed per request
//   - Default values are automatically applied for missing operations:
//     1. format: detected from alias extension first (if present), then from source path extension, fallback to "jpeg"
//     2. quality: 75
//     3. resize: fit="cover", fillColor="white"
//     4. crop and fit operations are optional
func ParseURL(path string) (*operations.Request, error) {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Split into parts
	parts := strings.SplitN(path, "/", 5)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid path format, expected /thumbs/[{signature}/]{size}/[filters:{filters}/]{path}[/as/{alias.ext}]")
	}

	// Check prefix
	if parts[0] != "thumbs" {
		return nil, fmt.Errorf("path must start with /thumbs")
	}

	req := &operations.Request{
		Operations: make([]operations.Operation, 0),
	}

	// Extract raw path to hash (everything after /thumbs/) for signature validation
	// Remove query parameters and fragments if present
	rawPath := path
	if idx := strings.IndexAny(path, "?#"); idx != -1 {
		rawPath = path[:idx]
	}
	// Store only the part after "thumbs/", without the prefix
	// RawURLPath format: {signature or size}/{size or filters}/{path}
	// This will be split in extractSignaturePayload to remove signature if present
	thumbsIdx := strings.Index(rawPath, "thumbs/")
	if thumbsIdx != -1 {
		req.RawURLPath = rawPath[thumbsIdx+7:] // +7 for "thumbs/"
	} else {
		req.RawURLPath = rawPath
	}

	// Determine signature presence by checking if parts[1] is a size format
	var sizeIndex int
	resizePrototype := operationRegistry.ResizeOp()

	if !resizePrototype.IsSizeFormat(parts[1]) {
		// Has signature
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid path format, expected /thumbs/{signature}/{size}/[filters:{filters}/]{path}[/as/{alias.ext}]")
		}
		sizeIndex = 2
		req.ProvidedSignature = parts[1]

		if !isValidSignature(req.ProvidedSignature) {
			return nil, fmt.Errorf("invalid signature format")
		}
	} else {
		// No signature
		sizeIndex = 1
		req.ProvidedSignature = ""
	}

	// Build canonical payload used for signature generation/verification.
	// Format: /{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
	req.SignaturePayload = extractSignaturePayloadFromRawPath(req.RawURLPath, req.ProvidedSignature)
	if req.SignaturePayload == "" {
		return nil, fmt.Errorf("unable to build signature payload from URL path")
	}

	// Clone resize operation for this request
	resizeOp := resizePrototype.Clone().(*operations.ResizeOperation)

	// Parse size
	if err := resizeOp.ParseSize(parts[sizeIndex]); err != nil {
		return nil, err
	}

	// Add resize operation to the list
	req.Operations = append(req.Operations, resizeOp)

	// Determine if we have filters
	var filePath string
	filterIndex := sizeIndex + 1

	if len(parts) > filterIndex && strings.HasPrefix(parts[filterIndex], "filters:") {
		// Parse filters
		filterString := strings.TrimPrefix(parts[filterIndex], "filters:")
		req.FilterString = filterString

		if len(parts) > filterIndex+1 {
			filePath = strings.Join(parts[filterIndex+1:], "/")
		} else {
			return nil, fmt.Errorf("missing file path after filters")
		}

		if err := parseFilters(filterString, req); err != nil {
			return nil, err
		}
	} else {
		// No filters
		filePath = strings.Join(parts[filterIndex:], "/")
		req.FilterString = ""
	}

	// Parse optional alias suffix: {path}/as/{alias.ext}
	sourcePath, aliasName, aliasFormat, hasAlias, err := parseAliasSuffix(filePath)
	if err != nil {
		return nil, err
	}
	req.Path = sourcePath
	req.Alias = aliasName
	req.AliasExtension = aliasFormat
	req.HasAlias = hasAlias

	// Ensure format operation is present (either from filters or from extension)
	if !hasOperation(req, "format") {
		formatOp := operationRegistry.FormatOp().Clone().(*operations.FormatOperation)
		if req.HasAlias && req.AliasExtension != "" {
			formatOp.Format = req.AliasExtension
		} else {
			formatOp.DetectFromExtension(req.Path)
		}
		req.Operations = append(req.Operations, formatOp)
	}

	// If alias extension is recognized and explicit format filter exists, it must match alias format.
	if req.HasAlias && req.AliasExtension != "" {
		if formatOp := getFormatOperation(req); formatOp != nil {
			normalizedFilterFormat := normalizeFormatName(formatOp.Format)
			if normalizedFilterFormat != req.AliasExtension {
				return nil, fmt.Errorf("alias extension %q conflicts with format(%s)", "."+req.AliasExtension, formatOp.Format)
			}
		}
	}

	// Ensure quality operation is present (either from filters or default)
	if !hasOperation(req, "quality") {
		qualityOp := operationRegistry.QualityOp().Clone().(*operations.QualityOperation)
		req.Operations = append(req.Operations, qualityOp)
	}

	// Apply fit mode from FitOperation to ResizeOperation if present
	applyFitModeToResize(req)

	// Run per-operation validation hooks
	if err := validateOperations(req.Operations); err != nil {
		return nil, err
	}

	// Validate crop and pcrop are not used together
	if err := validateCropExclusivity(req); err != nil {
		return nil, err
	}

	return req, nil
}

func parseAliasSuffix(path string) (sourcePath, aliasName, aliasFormat string, hasAlias bool, err error) {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[len(parts)-2] == "as" {
		hasAlias = true
		aliasName = strings.TrimSpace(parts[len(parts)-1])
		if aliasName == "" {
			return "", "", "", false, fmt.Errorf("alias filename is required after /as/")
		}

		aliasFormat = detectKnownFormat(aliasName)
		if aliasFormat == "" {
			return "", "", "", false, fmt.Errorf("alias must include a supported extension (.jpg, .jpeg, .png, .webp, .avif)")
		}

		sourcePath = strings.Join(parts[:len(parts)-2], "/")
		if strings.TrimSpace(sourcePath) == "" {
			return "", "", "", false, fmt.Errorf("source path is required before /as/{alias}")
		}

		return sourcePath, aliasName, aliasFormat, true, nil
	}

	return path, "", "", false, nil
}

func detectKnownFormat(path string) string {
	ext := strings.ToLower(path)
	switch {
	case strings.HasSuffix(ext, ".avif"):
		return "avif"
	case strings.HasSuffix(ext, ".webp"):
		return "webp"
	case strings.HasSuffix(ext, ".png"):
		return "png"
	case strings.HasSuffix(ext, ".jpg"), strings.HasSuffix(ext, ".jpeg"):
		return "jpeg"
	default:
		return ""
	}
}

func normalizeFormatName(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jpg" {
		return "jpeg"
	}
	return format
}

func getFormatOperation(req *operations.Request) *operations.FormatOperation {
	for _, op := range req.Operations {
		if formatOp, ok := op.(*operations.FormatOperation); ok {
			return formatOp
		}
	}
	return nil
}

// parseFilters parses semicolon-separated filters
func parseFilters(filterString string, req *operations.Request) error {
	filters := strings.Split(filterString, ";")

	// Track seen operation types to enforce one-per-type rule
	seenOperations := make(map[string]bool)

	for _, filter := range filters {
		filter = strings.TrimSpace(filter)
		if filter == "" {
			continue
		}

		// Parse the filter using registry
		op, err := operationRegistry.ParseFilter(filter)
		if err != nil {
			return err
		}

		// Check for duplicate operation types
		opName := op.Name()
		if seenOperations[opName] {
			return fmt.Errorf("duplicate operation '%s': only one %s operation allowed per request", opName, opName)
		}
		seenOperations[opName] = true

		// Add parsed operation to request
		req.Operations = append(req.Operations, op)
	}

	return nil
}

// hasOperation checks if an operation with the given name exists in the request
func hasOperation(req *operations.Request, name string) bool {
	for _, op := range req.Operations {
		if op.Name() == name {
			return true
		}
	}
	return false
}

// applyFitModeToResize updates ResizeOperation's Fit mode based on FitOperation if present
func applyFitModeToResize(req *operations.Request) {
	var resizeOp *operations.ResizeOperation
	var fitOp *operations.FitOperation
	var formatOp *operations.FormatOperation

	// Find resize, fit, and format operations
	for _, op := range req.Operations {
		switch v := op.(type) {
		case *operations.ResizeOperation:
			resizeOp = v
		case *operations.FitOperation:
			fitOp = v
		case *operations.FormatOperation:
			formatOp = v
		}
	}

	// If both resize and fit exist, apply fit mode and fill color to resize
	if resizeOp != nil && fitOp != nil {
		resizeOp.Fit = fitOp.Mode
		resizeOp.FillColor = fitOp.FillColor
	}

	// If fit and format exist, set format on fit for validation
	if fitOp != nil && formatOp != nil {
		fitOp.Format = formatOp.Format
	}
}

// validateCropExclusivity checks that crop and pcrop operations are not used together
func validateCropExclusivity(req *operations.Request) error {
	var hasCrop bool
	var hasPcrop bool

	for _, op := range req.Operations {
		switch op.(type) {
		case *operations.CropOperation:
			hasCrop = true
		case *operations.PercentCropOperation:
			hasPcrop = true
		}
	}

	if hasCrop && hasPcrop {
		return fmt.Errorf("cannot use both crop and pcrop operations in the same request (use either crop or pcrop, not both)")
	}

	return nil
}

func validateOperations(ops []operations.Operation) error {
	for _, op := range ops {
		if validatable, ok := op.(operations.Validatable); ok {
			if err := validatable.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// isValidSignature checks if the signature has a valid format
func isValidSignature(sig string) bool {
	if len(sig) < 8 || len(sig) > 64 {
		return false
	}
	for _, c := range sig {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func extractSignaturePayloadFromRawPath(rawPath, signature string) string {
	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	if parts[0] == signature && signature != "" {
		return "/" + parts[1]
	}

	return "/" + rawPath
}
