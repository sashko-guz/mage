package hashers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

const AlgorithmSHA256 = "sha256"

type sha256Hasher struct{}

func NewSHA256Hasher() Hasher {
	return &sha256Hasher{}
}

func (h *sha256Hasher) Name() string {
	return AlgorithmSHA256
}

func (h *sha256Hasher) DigestHexLength() int {
	return base64.RawURLEncoding.EncodedLen(sha256.Size)
}

func (h *sha256Hasher) ValidateRange(extractStart, extractLength int) error {
	return validateRange(h, extractStart, extractLength)
}

func (h *sha256Hasher) Generate(secretKey, payloadPath string, extractStart, extractLength int) string {
	normalizedPayload := normalizePayloadPath(payloadPath)

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(normalizedPayload))
	base64Digest := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	end := extractStart + extractLength
	return base64Digest[extractStart:end]
}
