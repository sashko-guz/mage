package parser

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ThumbnailRequest struct {
	Width             int
	Height            int
	Format            string
	Quality           int
	Fit               string
	Path              string
	ProvidedSignature string // Signature from URL
	FilterString      string // Raw filter string for signature validation
}

var (
	sizeRegex    = regexp.MustCompile(`^(\d+)x(\d+)$`)
	formatRegex  = regexp.MustCompile(`format\((\w+)\)`)
	qualityRegex = regexp.MustCompile(`quality\((\d+)\)`)
	fitRegex     = regexp.MustCompile(`fit\((\w+)\)`)
)

// ParseURL parses a URL path in the format:
// With signature (when secretKey configured): /thumbs/{signature}/{width}x{height}/[filters:{filters}/]{path}
// Without signature (when secretKey empty): /thumbs/{width}x{height}/[filters:{filters}/]{path}
// Signature is HMAC-SHA256 hash that validates all parameters after it.
// Filters are optional and must be prefixed with "filters:" if present. Multiple filters are separated by semicolons.
// Examples with signature and filters: /thumbs/a1b2c3d4e5f6g7h8/200x350/filters:format(webp);quality(88)/path-in-aws.jpeg
// Examples with signature, no filters: /thumbs/a1b2c3d4e5f6g7h8/200x350/image.jpg
// Examples without signature: /thumbs/200x350/image.jpg
//
//	/thumbs/200x350/filters:format(webp)/image.jpg
func ParseURL(path string, secretKey string) (*ThumbnailRequest, error) {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Split the path into parts
	// Format with signature: thumbs/{signature}/{size}/[filters:{filters}/]{path}
	// Format without signature: thumbs/{size}/[filters:{filters}/]{path}
	parts := strings.SplitN(path, "/", 5)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid path format, expected /thumbs/[{signature}/]{size}/[filters:{filters}/]{path}")
	}

	// Check if it starts with "thumbs"
	if parts[0] != "thumbs" {
		return nil, fmt.Errorf("path must start with /thumbs")
	}

	req := &ThumbnailRequest{
		Quality: 75,      // Default quality
		Format:  "jpeg",  // Default format
		Fit:     "cover", // Default fit mode - scales and crops to exact dimensions
	}

	// Determine if we have a signature by checking if parts[1] matches size format
	// If parts[1] is in format NxN (e.g., "200x350"), then no signature
	// Otherwise, parts[1] is signature and parts[2] is size
	var sizeIndex int
	hasSignatureInURL := !sizeRegex.MatchString(parts[1])

	if hasSignatureInURL {
		// Has signature - parts[1] is signature, parts[2] is size
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid path format, expected /thumbs/{signature}/{size}/[filters:{filters}/]{path}")
		}
		sizeIndex = 2
		req.ProvidedSignature = parts[1]

		// Validate signature format (should be hex string)
		if !isValidSignature(req.ProvidedSignature) {
			return nil, fmt.Errorf("invalid signature format")
		}
	} else {
		// No signature - parts[1] is the size
		sizeIndex = 1
		req.ProvidedSignature = ""
	}

	// Parse size (e.g., "200x350")
	sizeMatch := sizeRegex.FindStringSubmatch(parts[sizeIndex])
	if sizeMatch == nil {
		return nil, fmt.Errorf("invalid size format, expected {width}x{height}")
	}

	var err error
	req.Width, err = strconv.Atoi(sizeMatch[1])
	if err != nil {
		return nil, fmt.Errorf("invalid width: %w", err)
	}

	req.Height, err = strconv.Atoi(sizeMatch[2])
	if err != nil {
		return nil, fmt.Errorf("invalid height: %w", err)
	}

	// Determine if we have filters or not
	// Filters must start with "filters:" prefix
	var filePath string
	filterIndex := sizeIndex + 1

	if len(parts) > filterIndex && strings.HasPrefix(parts[filterIndex], "filters:") {
		// We have filters
		filterString := strings.TrimPrefix(parts[filterIndex], "filters:")
		req.FilterString = filterString // Store for signature validation
		if len(parts) > filterIndex+1 {
			filePath = strings.Join(parts[filterIndex+1:], "/")
		} else {
			return nil, fmt.Errorf("missing file path after filters")
		}
		parseFilters(filterString, req)
	} else {
		// No filters, everything from current index onwards is the path
		// This handles paths with multiple directories like: one/two/file.jpeg
		filePath = strings.Join(parts[filterIndex:], "/")
		req.FilterString = "" // Empty filter string for signature
		// Extract format from file extension
		detectFormatFromPath(filePath, req)
	}

	req.Path = filePath

	return req, nil
}

// parseFilters parses filter string with semicolon-separated filters
func parseFilters(filterString string, req *ThumbnailRequest) {
	filters := strings.Split(filterString, ";")

	for _, filter := range filters {
		filter = strings.TrimSpace(filter)

		// Extract format
		if formatMatch := formatRegex.FindStringSubmatch(filter); formatMatch != nil {
			req.Format = strings.ToLower(formatMatch[1])
		}

		// Extract quality
		if qualityMatch := qualityRegex.FindStringSubmatch(filter); qualityMatch != nil {
			quality, err := strconv.Atoi(qualityMatch[1])
			if err == nil && quality >= 1 && quality <= 100 {
				req.Quality = quality
			}
		}

		// Extract fit mode
		if fitMatch := fitRegex.FindStringSubmatch(filter); fitMatch != nil {
			fitMode := strings.ToLower(fitMatch[1])
			if fitMode == "fill" || fitMode == "cover" {
				req.Fit = fitMode
			}
		}
	}
}

// detectFormatFromPath extracts format from file extension
func detectFormatFromPath(filePath string, req *ThumbnailRequest) {
	ext := filepath.Ext(filePath)
	if ext != "" {
		ext = strings.TrimPrefix(ext, ".")
		ext = strings.ToLower(ext)
		// Map extensions to format names
		switch ext {
		case "jpg":
			req.Format = "jpeg"
		case "jpeg", "png", "webp", "gif":
			req.Format = ext
		}
	}
}

// isValidSignature checks if the signature has a valid format
func isValidSignature(sig string) bool {
	// Signature should be a hex string (typically 16 characters)
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

// GenerateSignature creates HMAC-SHA256 signature for the request parameters
// Payload includes: width:height:filterString:path
// Secret key is taken from SIGNATURE_SECRET_KEY environment variable
func GenerateSignature(width, height int, filterString, path, secretKey string) string {
	// Build payload from all parameters that need protection
	payload := fmt.Sprintf("%d:%d:%s:%s",
		width,
		height,
		filterString, // Empty string if no filters
		path,
	)

	// Create HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(payload))

	// Return first 16 hex chars (64-bit hash)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// VerifySignature validates that the provided signature matches the expected signature
func VerifySignature(req *ThumbnailRequest, secretKey string) bool {
	if secretKey == "" {
		return req.ProvidedSignature == ""
	}

	if req.ProvidedSignature == "" {
		// If signature validation is enabled but no signature provided, reject
		return false
	}

	// Generate expected signature from request parameters
	expectedSignature := GenerateSignature(
		req.Width,
		req.Height,
		req.FilterString,
		req.Path,
		secretKey,
	)

	fmt.Printf("Expected signature: %s, Provided signature: %s\n", expectedSignature, req.ProvidedSignature)

	// Compare provided signature with expected
	return req.ProvidedSignature == expectedSignature
}

// GetCachePath generates hierarchical filesystem path for caching
// Uses nginx-style distribution: last 3 chars of signature create 3-level hierarchy
// Example: signature "a1b2c3d4e5f6g7h8" → basePath/8/7/h/a1b2c3d4e5f6g7h8/200x350/path/image.jpg
func GetCachePath(basePath string, req *ThumbnailRequest) string {
	sig := req.ProvidedSignature
	len := len(sig)

	// Use last 3 characters for hierarchical distribution (similar to nginx cache)
	level1 := sig[len-1:]        // last char
	level2 := sig[len-2 : len-1] // second to last
	level3 := sig[len-3 : len-2] // third to last

	// Build hierarchical path: base/l1/l2/l3/signature/dimensions/imagePath
	dimensionDir := fmt.Sprintf("%dx%d", req.Width, req.Height)

	return filepath.Join(basePath, level1, level2, level3, sig, dimensionDir, req.Path)
}

// GenerateURL creates a signed URL from request parameters
// Useful for generating URLs programmatically
// If secretKey is empty, signature is omitted from the URL
func GenerateURL(width, height int, filterString, path, secretKey string) string {
	if secretKey == "" {
		// No signature - generate URL without signature
		if filterString != "" {
			return fmt.Sprintf("/thumbs/%dx%d/filters:%s/%s",
				width, height, filterString, path)
		}
		return fmt.Sprintf("/thumbs/%dx%d/%s",
			width, height, path)
	}

	// Generate signature
	signature := GenerateSignature(width, height, filterString, path, secretKey)

	// Build URL with signature
	if filterString != "" {
		return fmt.Sprintf("/thumbs/%s/%dx%d/filters:%s/%s",
			signature, width, height, filterString, path)
	}
	return fmt.Sprintf("/thumbs/%s/%dx%d/%s",
		signature, width, height, path)
}
