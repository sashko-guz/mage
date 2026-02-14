package operations

import (
	"fmt"
	"strings"

	"github.com/cshum/vipsgen/vips"
)

// PercentCropOperation handles pcrop(x1,y1,x2,y2) filter where coordinates are 0-100 percentages
type PercentCropOperation struct {
	X1, Y1, X2, Y2 int // Values 0-100 representing percentages
	enabled        bool
}

func NewPercentCropOperation() *PercentCropOperation {
	return &PercentCropOperation{}
}

func (o *PercentCropOperation) Name() string {
	return "pcrop"
}

func (o *PercentCropOperation) Clone() Operation {
	return NewPercentCropOperation()
}

func (o *PercentCropOperation) Parse(filter string) (bool, error) {
	if !strings.HasPrefix(filter, "pcrop(") {
		return false, nil
	}

	if !strings.HasSuffix(filter, ")") {
		return false, fmt.Errorf("pcrop filter missing closing parenthesis")
	}

	content := filter[6 : len(filter)-1]
	parts := strings.Split(content, ",")
	if len(parts) != 4 {
		return false, fmt.Errorf("pcrop filter expects 4 coordinates (x1,y1,x2,y2) as percentages 0-100, got: %s", content)
	}

	coords := make([]int, 4)
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return false, fmt.Errorf("pcrop coordinate %d is empty", i+1)
		}

		num := 0
		negative := false
		for j, ch := range part {
			if j == 0 && ch == '-' {
				negative = true
				continue
			}
			if ch < '0' || ch > '9' {
				return false, fmt.Errorf("pcrop coordinate %d must be a number, got: %s", i+1, part)
			}
			num = num*10 + int(ch-'0')
		}

		if negative {
			num = -num
		}
		coords[i] = num
	}

	o.X1, o.Y1, o.X2, o.Y2 = coords[0], coords[1], coords[2], coords[3]

	// Validate percentages are in range 0-100
	if o.X1 < 0 || o.Y1 < 0 || o.X2 < 0 || o.Y2 < 0 {
		return false, fmt.Errorf("invalid pcrop coordinates: percentages must be >= 0 (got pcrop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	if o.X1 > 100 || o.Y1 > 100 || o.X2 > 100 || o.Y2 > 100 {
		return false, fmt.Errorf("invalid pcrop coordinates: percentages must be <= 100 (got pcrop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	if o.X2 <= o.X1 {
		return false, fmt.Errorf("invalid pcrop coordinates: x2 must be greater than x1 (got pcrop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	if o.Y2 <= o.Y1 {
		return false, fmt.Errorf("invalid pcrop coordinates: y2 must be greater than y1 (got pcrop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	o.enabled = true
	return true, nil
}

func (o *PercentCropOperation) Apply(img *vips.Image) (*vips.Image, error) {
	if !o.enabled {
		return img, nil
	}

	imgWidth := img.Width()
	imgHeight := img.Height()

	// Convert percentages to pixel coordinates
	cropLeft := (imgWidth * o.X1) / 100
	cropTop := (imgHeight * o.Y1) / 100
	cropRight := (imgWidth * o.X2) / 100
	cropBottom := (imgHeight * o.Y2) / 100

	cropWidth := cropRight - cropLeft
	cropHeight := cropBottom - cropTop

	// Validate calculated coordinates are within image bounds
	if cropLeft < 0 || cropTop < 0 || cropRight > imgWidth || cropBottom > imgHeight {
		return nil, fmt.Errorf("pcrop percentages result in out of bounds area: image is %dx%d, calculated crop area is (%d,%d) to (%d,%d)",
			imgWidth, imgHeight, cropLeft, cropTop, cropRight, cropBottom)
	}

	if cropWidth <= 0 || cropHeight <= 0 {
		return nil, fmt.Errorf("invalid pcrop: resulting crop area is invalid (%dx%d)", cropWidth, cropHeight)
	}

	err := img.ExtractArea(cropLeft, cropTop, cropWidth, cropHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to percentage crop image: %w", err)
	}

	return img, nil
}
