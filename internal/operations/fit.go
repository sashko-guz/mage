package operations

import (
	"fmt"
	"strings"

	"github.com/cshum/vipsgen/vips"
)

// FitOperation handles fit(mode[,color]) filter
type FitOperation struct {
	Mode      string
	FillColor string
	Format    string // Set during validation to check transparent compatibility
}

func NewFitOperation() *FitOperation {
	return &FitOperation{}
}

func (o *FitOperation) Name() string {
	return "fit"
}

func (o *FitOperation) Clone() Operation {
	return NewFitOperation()
}

func (o *FitOperation) Parse(filter string) (bool, error) {
	if !strings.HasPrefix(filter, "fit(") {
		return false, nil
	}

	if !strings.HasSuffix(filter, ")") {
		return false, fmt.Errorf("fit filter missing closing parenthesis")
	}

	content := filter[4 : len(filter)-1]
	parts := strings.Split(content, ",")
	if len(parts) == 0 || len(parts) > 2 {
		return false, fmt.Errorf("fit filter expects 1 or 2 parameters, got: %s", content)
	}

	fitMode := strings.TrimSpace(strings.ToLower(parts[0]))
	if fitMode != "fill" && fitMode != "cover" {
		return false, fmt.Errorf("fit mode must be 'fill' or 'cover', got: %s", fitMode)
	}

	o.Mode = fitMode
	o.FillColor = "white" // Default

	if len(parts) == 2 {
		if fitMode != "fill" {
			return false, fmt.Errorf("color parameter is only valid for fit(fill), not fit(%s)", fitMode)
		}

		color := strings.TrimSpace(strings.ToLower(parts[1]))
		switch color {
		case "black", "white", "transparent":
			o.FillColor = color
		default:
			return false, fmt.Errorf("fill color must be 'black', 'white', or 'transparent', got: %s", color)
		}
	}

	return true, nil
}

func (o *FitOperation) Apply(img *vips.Image) (*vips.Image, error) {
	// Fit is applied by modifying ResizeOperation behavior
	// This operation just stores the preference
	return img, nil
}

// Validate checks that transparent fill color is only used with PNG, WebP, or AVIF formats
func (o *FitOperation) Validate() error {
	if o.FillColor == "transparent" {
		// Transparent requires PNG, WebP, or AVIF format
		if o.Format != "png" && o.Format != "webp" && o.Format != "avif" {
			return fmt.Errorf("transparent fill color requires PNG, WebP, or AVIF format (use filters:format(png), filters:format(webp), or filters:format(avif))")
		}
	}
	return nil
}
