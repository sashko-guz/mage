package signature

import (
	"fmt"
	"strings"

	"github.com/sashko-guz/mage/internal/logger"
	"github.com/sashko-guz/mage/internal/operations"
	"github.com/sashko-guz/mage/internal/signature/hashers"
)

const (
	AlgorithmSHA256 = "sha256"
	AlgorithmSHA512 = "sha512"
)

type Config struct {
	SecretKey     string
	Algorithm     string
	ExtractStart  int
	ExtractLength int
}

// Signature handles URL signature generation and verification using configurable HMAC algorithms.
type Signature struct {
	secretKey     string
	hasher        hashers.Hasher
	extractStart  int
	extractLength int
}

// New creates a new Signature with normalized config values.
// If SecretKey is empty, signature validation is disabled.
func New(cfg Config) (*Signature, error) {
	hasher, err := newHasher(cfg.Algorithm)
	if err != nil {
		return nil, err
	}

	if err := hasher.ValidateRange(cfg.ExtractStart, cfg.ExtractLength); err != nil {
		return nil, err
	}

	return &Signature{
		secretKey:     cfg.SecretKey,
		hasher:        hasher,
		extractStart:  cfg.ExtractStart,
		extractLength: cfg.ExtractLength,
	}, nil
}

// IsEnabled returns true if signature validation is enabled.
func (s *Signature) IsEnabled() bool {
	return s.secretKey != ""
}

// Generate creates signature from payload path only.
// Payload format: /{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
func (s *Signature) Generate(payloadPath string) string {
	if !s.IsEnabled() {
		return ""
	}

	return s.hasher.Generate(s.secretKey, payloadPath, s.extractStart, s.extractLength)
}

// Verify validates that the provided signature matches the expected signature.
func (s *Signature) Verify(req *operations.Request) error {
	if !s.IsEnabled() {
		if req.ProvidedSignature != "" {
			return fmt.Errorf("signature provided in URL but signature validation is not configured on server: got %s", req.ProvidedSignature)
		}
		return nil
	}

	if req.ProvidedSignature == "" {
		return fmt.Errorf("signature required but not provided in URL")
	}

	payloadToHash := req.SignaturePayload
	if payloadToHash == "" {
		return fmt.Errorf("signature payload is empty")
	}

	expectedSignature := s.hasher.Generate(s.secretKey, payloadToHash, s.extractStart, s.extractLength)

	if req.ProvidedSignature != expectedSignature {
		logger.Debugf("[Signature] Signature mismatch: expected=%s, provided=%s, payload=%s", expectedSignature, req.ProvidedSignature, payloadToHash)
		return fmt.Errorf("invalid signature: got %s", req.ProvidedSignature)
	}

	return nil
}

func newHasher(algorithm string) (hashers.Hasher, error) {
	normalized := strings.ToLower(strings.TrimSpace(algorithm))
	switch normalized {
	case AlgorithmSHA256:
		return hashers.NewSHA256Hasher(), nil
	case AlgorithmSHA512:
		return hashers.NewSHA512Hasher(), nil
	default:
		return nil, fmt.Errorf("invalid signature algorithm %q: expected %q or %q", algorithm, AlgorithmSHA256, AlgorithmSHA512)
	}
}
