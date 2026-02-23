package hashers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
)

const AlgorithmSHA512 = "sha512"

type sha512Hasher struct{}

func NewSHA512Hasher() Hasher {
	return &sha512Hasher{}
}

func (h *sha512Hasher) Name() string {
	return AlgorithmSHA512
}

func (h *sha512Hasher) DigestHexLength() int {
	return sha512.Size * 2
}

func (h *sha512Hasher) ValidateRange(extractStart, extractLength int) error {
	return validateRange(h, extractStart, extractLength)
}

func (h *sha512Hasher) Generate(secretKey, payloadPath string, extractStart, extractLength int) string {
	normalizedPayload := normalizePayloadPath(payloadPath)

	mac := hmac.New(sha512.New, []byte(secretKey))
	mac.Write([]byte(normalizedPayload))
	hexDigest := hex.EncodeToString(mac.Sum(nil))

	end := extractStart + extractLength
	return hexDigest[extractStart:end]
}
