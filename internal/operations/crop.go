package operations

import (
	"fmt"
	"strings"

	"github.com/cshum/vipsgen/vips"
)

// CropOperation handles crop(x1,y1,x2,y2) filter
type CropOperation struct {
	X1, Y1, X2, Y2 int
	enabled        bool
}

func NewCropOperation() *CropOperation {
	return &CropOperation{}
}

func (o *CropOperation) Name() string {
	return "crop"
}

func (o *CropOperation) Clone() Operation {
	return NewCropOperation()
}

func (o *CropOperation) Parse(filter string) (bool, error) {
	if !strings.HasPrefix(filter, "crop(") {
		return false, nil
	}

	if !strings.HasSuffix(filter, ")") {
		return false, fmt.Errorf("crop filter missing closing parenthesis")
	}

	content := filter[5 : len(filter)-1]
	parts := strings.Split(content, ",")
	if len(parts) != 4 {
		return false, fmt.Errorf("crop filter expects 4 coordinates (x1,y1,x2,y2), got: %s", content)
	}

	coords := make([]int, 4)
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return false, fmt.Errorf("crop coordinate %d is empty", i+1)
		}

		num := 0
		negative := false
		for j, ch := range part {
			if j == 0 && ch == '-' {
				negative = true
				continue
			}
			if ch < '0' || ch > '9' {
				return false, fmt.Errorf("crop coordinate %d must be a number, got: %s", i+1, part)
			}
			num = num*10 + int(ch-'0')
		}

		if negative {
			num = -num
		}
		coords[i] = num
	}

	o.X1, o.Y1, o.X2, o.Y2 = coords[0], coords[1], coords[2], coords[3]

	if o.X1 < 0 || o.Y1 < 0 || o.X2 < 0 || o.Y2 < 0 {
		return false, fmt.Errorf("invalid crop coordinates: negative values not allowed (got crop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	if o.X2 <= o.X1 {
		return false, fmt.Errorf("invalid crop coordinates: x2 must be greater than x1 (got crop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	if o.Y2 <= o.Y1 {
		return false, fmt.Errorf("invalid crop coordinates: y2 must be greater than y1 (got crop(%d,%d,%d,%d))", o.X1, o.Y1, o.X2, o.Y2)
	}

	cropWidth := o.X2 - o.X1
	cropHeight := o.Y2 - o.Y1
	if cropWidth == 0 || cropHeight == 0 {
		return false, fmt.Errorf("invalid crop coordinates: crop area cannot be zero-sized (got %dx%d)", cropWidth, cropHeight)
	}

	o.enabled = true
	return true, nil
}

func (o *CropOperation) Apply(img *vips.Image) (*vips.Image, error) {
	if !o.enabled {
		return img, nil
	}

	cropLeft := o.X1
	cropTop := o.Y1
	cropWidth := o.X2 - o.X1
	cropHeight := o.Y2 - o.Y1

	// Validate crop coordinates are within image bounds
	imgWidth := img.Width()
	imgHeight := img.Height()

	if cropLeft < 0 || cropTop < 0 || cropLeft >= imgWidth || cropTop >= imgHeight ||
		o.X2 > imgWidth || o.Y2 > imgHeight {
		return nil, fmt.Errorf("crop coordinates out of bounds: image is %dx%d, crop area is (%d,%d) to (%d,%d)",
			imgWidth, imgHeight, cropLeft, cropTop, o.X2, o.Y2)
	}

	err := img.ExtractArea(cropLeft, cropTop, cropWidth, cropHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to crop image: %w", err)
	}

	return img, nil
}
