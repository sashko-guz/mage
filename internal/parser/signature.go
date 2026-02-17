package parser

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/sashko-guz/mage/internal/logger"
	"github.com/sashko-guz/mage/internal/operations"
)

// Signature handles URL signature generation and verification using HMAC-SHA256
type Signature struct {
	secretKey string
}

// NewSignature creates a new Signature with the given secret key
// If secretKey is empty, signature validation is disabled
func NewSignature(secretKey string) *Signature {
	return &Signature{
		secretKey: secretKey,
	}
}

// IsEnabled returns true if signature validation is enabled
func (s *Signature) IsEnabled() bool {
	return s.secretKey != ""
}

// Generate creates HMAC-SHA256 signature for the request parameters
// Payload is the raw URL path after the signature: /{size}/[filters:{filters}/]{path}
// Width/height can be nil for aspect ratio preserving sizes (rendered as "x" in URL)
func (s *Signature) Generate(width, height *int, filterString, path string) string {
	if !s.IsEnabled() {
		return ""
	}

	// Build the size string for the URL
	sizeStr := formatSizeString(width, height)

	// Build the raw URL path that will be hashed
	// This must match what Verify() expects to hash
	var payloadPath string
	if filterString != "" {
		payloadPath = fmt.Sprintf("/%s/filters:%s/%s", sizeStr, filterString, path)
	} else {
		payloadPath = fmt.Sprintf("/%s/%s", sizeStr, path)
	}

	// Create HMAC-SHA256 of the raw URL path
	h := hmac.New(sha256.New, []byte(s.secretKey))
	h.Write([]byte(payloadPath))

	// Return first 16 hex chars (64-bit hash)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Verify validates that the provided signature matches the expected signature
// Uses the raw URL path (everything after /thumbs/) for signature validation,
// making it resilient to future changes in URL parsing logic.
// Returns an error with context if validation fails, nil if valid
func (s *Signature) Verify(req *operations.Request) error {
	if !s.IsEnabled() {
		// Signature validation is disabled
		if req.ProvidedSignature != "" {
			return fmt.Errorf("signature provided in URL but signature validation is not configured on server: got %s", req.ProvidedSignature)
		}
		return nil // No signature required, no signature provided - OK
	}

	if req.ProvidedSignature == "" {
		// Signature validation is enabled but no signature provided
		return fmt.Errorf("signature required but not provided in URL")
	}

	// Verify signature by hashing the raw URL path
	// This extracts what comes after the signature itself in the URL
	// Format: {signature}/{size}/[filters:{filters}/]{path}
	// We hash: {size}/[filters:{filters}/]{path}
	payloadToHash := extractSignaturePayload(req.RawURLPath, req.ProvidedSignature)
	if payloadToHash == "" {
		return fmt.Errorf("unable to extract payload for signature verification from URL path: %s", req.RawURLPath)
	}

	// Create HMAC-SHA256 of the raw payload
	h := hmac.New(sha256.New, []byte(s.secretKey))
	h.Write([]byte(payloadToHash))
	expectedSignature := hex.EncodeToString(h.Sum(nil))[:16]

	// Compare provided signature with expected
	if req.ProvidedSignature != expectedSignature {
		logger.Debugf("[Signature] Signature mismatch: expected=%s, provided=%s, payload=%s", expectedSignature, req.ProvidedSignature, payloadToHash)

		return fmt.Errorf("invalid signature: got %s", req.ProvidedSignature)
	}

	return nil // Signature is valid
}

// extractSignaturePayload extracts the portion of the URL path that should be hashed
// for signature validation. Removes the signature token itself from the raw path.
// Input format: {signature}/{size}/[filters:{filters}/]{path}
// Output: /{size}/[filters:{filters}/]{path} (includes leading slash)
func extractSignaturePayload(rawPath, signature string) string {
	// rawPath is in format: signature-or-size/size/[filters]/path
	// We need to find where the signature ends and the actual payload begins
	// The signature is the first path segment, so we skip it and return the rest

	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	firstPart := parts[0]
	// If first part matches the signature, skip it and return the rest with leading slash
	if firstPart == signature {
		return "/" + parts[1]
	}

	// If first part doesn't match signature, then there's no signature in URL
	// and we should return the whole path with leading slash (this happens for URLs without signatures)
	return "/" + rawPath
}

// GenerateURL creates a signed URL from request parameters
// If signature is disabled, signature is omitted from the URL
// Width/height can be nil for aspect ratio preserving sizes
func (s *Signature) GenerateURL(width, height *int, filterString, path string) string {
	// Build size string
	sizeStr := formatSizeString(width, height)

	if !s.IsEnabled() {
		// No signature - generate URL without signature
		if filterString != "" {
			return fmt.Sprintf("/thumbs/%s/filters:%s/%s",
				sizeStr, filterString, path)
		}
		return fmt.Sprintf("/thumbs/%s/%s",
			sizeStr, path)
	}

	// Generate signature
	signature := s.Generate(width, height, filterString, path)

	// Build URL with signature
	if filterString != "" {
		return fmt.Sprintf("/thumbs/%s/%s/filters:%s/%s",
			signature, sizeStr, filterString, path)
	}
	return fmt.Sprintf("/thumbs/%s/%s/%s",
		signature, sizeStr, path)
}

// formatSizeString converts optional width/height to URL size string
func formatSizeString(width, height *int) string {
	if width == nil && height == nil {
		return "x"
	}
	if width == nil {
		return fmt.Sprintf("x%d", *height)
	}
	if height == nil {
		return fmt.Sprintf("%dx", *width)
	}
	return fmt.Sprintf("%dx%d", *width, *height)
}
