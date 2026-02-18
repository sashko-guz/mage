package parser

import (
	"fmt"
	"strings"

	"github.com/sashko-guz/mage/internal/operations"
)

var (
	// Global operation registry - initialized once at startup
	operationRegistry *operations.Registry
)

// init registers all available operations
func init() {
	operationRegistry = operations.NewRegistry()
}

// ParseURL parses a URL path and returns a Request with parsed operations
//
// URL Format:
//   - With signature: /thumbs/{signature}/{size}/[filters:{filters}/]{path}
//   - Without signature: /thumbs/{size}/[filters:{filters}/]{path}
//
// Examples:
//   - /thumbs/200x350/filters:format(webp);quality(88)/image.jpg
//   - /thumbs/abc123/200x300/filters:crop(10,10,500,500);fit(cover)/image.jpg
//
// Operation Rules:
//   - Only ONE operation of each type is allowed per request
//   - Default values are automatically applied for missing operations:
//     1. format: detected from file extension, fallback to "jpeg"
//     2. quality: 75
//     3. resize: fit="cover", fillColor="white"
//     4. crop and fit operations are optional
func ParseURL(path string) (*operations.Request, error) {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Split into parts
	parts := strings.SplitN(path, "/", 5)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid path format, expected /thumbs/[{signature}/]{size}/[filters:{filters}/]{path}")
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
			return nil, fmt.Errorf("invalid path format, expected /thumbs/{signature}/{size}/[filters:{filters}/]{path}")
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

	// Ensure format operation is present (either from filters or from extension)
	if !hasOperation(req, "format") {
		formatOp := operationRegistry.FormatOp().Clone().(*operations.FormatOperation)
		formatOp.DetectFromExtension(filePath)
		req.Operations = append(req.Operations, formatOp)
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

	// Validate transparent fill color only works with PNG format
	if err := validateTransparentFill(req); err != nil {
		return nil, err
	}

	// Validate crop and pcrop are not used together
	if err := validateCropExclusivity(req); err != nil {
		return nil, err
	}

	req.Path = filePath

	return req, nil
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

	// Find resize and fit operations
	for _, op := range req.Operations {
		switch v := op.(type) {
		case *operations.ResizeOperation:
			resizeOp = v
		case *operations.FitOperation:
			fitOp = v
		}
	}

	// If both exist, apply fit mode and fill color to resize
	if resizeOp != nil && fitOp != nil {
		resizeOp.Fit = fitOp.Mode
		resizeOp.FillColor = fitOp.FillColor
	}
}

// validateTransparentFill checks that transparent fill color is only used with PNG format
func validateTransparentFill(req *operations.Request) error {
	var resizeOp *operations.ResizeOperation
	var formatOp *operations.FormatOperation

	// Find resize and format operations
	for _, op := range req.Operations {
		switch v := op.(type) {
		case *operations.ResizeOperation:
			resizeOp = v
		case *operations.FormatOperation:
			formatOp = v
		}
	}

	// Check if transparent fill is used
	if resizeOp != nil && resizeOp.FillColor == "transparent" {
		// Transparent requires PNG format
		if formatOp == nil || formatOp.Format != "png" {
			return fmt.Errorf("transparent fill color requires PNG format (use filters:format(png))")
		}
	}

	return nil
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
