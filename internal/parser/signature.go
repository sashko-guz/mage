package parser

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

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

// Generate creates signature from payload path only.
// Payload format: /{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
func (s *Signature) Generate(payloadPath string) string {
	if !s.IsEnabled() {
		return ""
	}

	return signPayloadPath(s.secretKey, payloadPath)
}

// Verify validates that the provided signature matches the expected signature
// Uses parser-provided canonical signature payload for validation.
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

	payloadToHash := req.SignaturePayload
	if payloadToHash == "" {
		return fmt.Errorf("signature payload is empty")
	}

	expectedSignature := signPayloadPath(s.secretKey, payloadToHash)

	// Compare provided signature with expected
	if req.ProvidedSignature != expectedSignature {
		logger.Debugf("[Signature] Signature mismatch: expected=%s, provided=%s, payload=%s", expectedSignature, req.ProvidedSignature, payloadToHash)

		return fmt.Errorf("invalid signature: got %s", req.ProvidedSignature)
	}

	return nil // Signature is valid
}

func signPayloadPath(secretKey, payloadPath string) string {
	if payloadPath == "" || payloadPath[0] != '/' {
		payloadPath = "/" + payloadPath
	}

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(payloadPath))

	return hex.EncodeToString(h.Sum(nil))[:16]
}
