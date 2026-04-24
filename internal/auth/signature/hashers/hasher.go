package hashers

import "fmt"

type Hasher interface {
	Name() string
	DigestHexLength() int
	Generate(secretKey, payloadPath string, extractStart, extractLength int) string
	ValidateRange(extractStart, extractLength int) error
}

func validateRange(hasher Hasher, extractStart, extractLength int) error {
	if extractStart < 0 {
		return fmt.Errorf("invalid signature extract start %d: must be >= 0", extractStart)
	}

	if extractLength <= 0 {
		return fmt.Errorf("invalid signature length %d: must be > 0", extractLength)
	}

	digestHexLen := hasher.DigestHexLength()
	if extractStart >= digestHexLen {
		return fmt.Errorf("invalid signature extract start %d: exceeds %s digest hex length %d", extractStart, hasher.Name(), digestHexLen)
	}

	if extractStart+extractLength > digestHexLen {
		return fmt.Errorf("invalid signature range start=%d length=%d: exceeds %s digest hex length %d", extractStart, extractLength, hasher.Name(), digestHexLen)
	}

	return nil
}

func normalizePayloadPath(payloadPath string) string {
	if payloadPath == "" || payloadPath[0] != '/' {
		return "/" + payloadPath
	}
	return payloadPath
}
