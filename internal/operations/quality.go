package operations

import (
	"fmt"
	"strings"

	"github.com/cshum/vipsgen/vips"
)

// QualityOperation handles quality(N) filter
type QualityOperation struct {
	Quality int
}

func NewQualityOperation() *QualityOperation {
	return &QualityOperation{
		Quality: 75, // Default quality
	}
}

func (o *QualityOperation) Name() string {
	return "quality"
}

func (o *QualityOperation) Clone() Operation {
	return NewQualityOperation()
}

func (o *QualityOperation) Parse(filter string) (bool, error) {
	if !strings.HasPrefix(filter, "quality(") {
		return false, nil
	}

	if !strings.HasSuffix(filter, ")") {
		return false, fmt.Errorf("quality filter missing closing parenthesis")
	}

	content := filter[8 : len(filter)-1]
	content = strings.TrimSpace(content)

	if content == "" {
		return false, fmt.Errorf("quality filter requires a value")
	}

	quality := 0
	for _, ch := range content {
		if ch < '0' || ch > '9' {
			return false, fmt.Errorf("quality must be a number, got: %s", content)
		}
		quality = quality*10 + int(ch-'0')
	}

	if quality < 1 || quality > 100 {
		return false, fmt.Errorf("quality must be between 1 and 100, got: %d", quality)
	}

	o.Quality = quality
	return true, nil
}

func (o *QualityOperation) Apply(img *vips.Image) (*vips.Image, error) {
	// Quality is applied during export
	return img, nil
}
