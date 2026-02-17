package operations

import (
	"fmt"
	"strings"

	"github.com/cshum/vipsgen/vips"
)

// ResizeOperation handles WxH size format and resizing
type ResizeOperation struct {
	Width     *int
	Height    *int
	Fit       string // "cover" or "fill"
	FillColor string // Color for fill mode (default "white")
}

func NewResizeOperation() *ResizeOperation {
	return &ResizeOperation{
		Fit:       "cover", // Default fit mode
		FillColor: "white", // Default fill color
	}
}

func (o *ResizeOperation) Name() string {
	return "resize"
}

func (o *ResizeOperation) Clone() Operation {
	return NewResizeOperation()
}

// ParseSize parses size string like "200x300", "200x", "x300", or "x"
func (o *ResizeOperation) ParseSize(sizeStr string) error {
	xIndex := strings.IndexByte(sizeStr, 'x')
	if xIndex == -1 {
		return fmt.Errorf("invalid size format, expected {width}x{height}, x{height}, {width}x, or x")
	}

	widthStr := sizeStr[:xIndex]
	heightStr := sizeStr[xIndex+1:]

	if widthStr == "" && heightStr == "" {
		o.Width = nil
		o.Height = nil
		return nil
	}

	if widthStr != "" {
		width, err := parsePositiveInt(widthStr)
		if err != nil {
			return fmt.Errorf("invalid width: %w", err)
		}
		o.Width = &width
	}

	if heightStr != "" {
		height, err := parsePositiveInt(heightStr)
		if err != nil {
			return fmt.Errorf("invalid height: %w", err)
		}
		o.Height = &height
	}

	return nil
}

func (o *ResizeOperation) Parse(filter string) (bool, error) {
	// Resize is not a filter, parsed separately
	return false, nil
}

func (o *ResizeOperation) IsSizeFormat(str string) bool {
	return strings.IndexByte(str, 'x') != -1
}

func (o *ResizeOperation) Apply(img *vips.Image) (*vips.Image, error) {
	// If no size specified, return original
	if o.Width == nil && o.Height == nil {
		return img, nil
	}

	// Height only - maintain aspect ratio
	if o.Width == nil {
		return o.resizeHeightOnly(img, *o.Height)
	}

	// Width only - maintain aspect ratio
	if o.Height == nil {
		return o.resizeWidthOnly(img, *o.Width)
	}

	// Both width and height - use fit mode (handled by FitOperation)
	return o.resizeBoth(img, *o.Width, *o.Height)
}

func (o *ResizeOperation) resizeHeightOnly(img *vips.Image, height int) (*vips.Image, error) {
	currentHeight := img.Height()
	if currentHeight <= 0 {
		return nil, fmt.Errorf("failed to resize by height: invalid source height %d", currentHeight)
	}
	if height == currentHeight {
		return img, nil
	}

	scale := float64(height) / float64(currentHeight)
	options := vips.DefaultResizeOptions()
	options.Vscale = scale
	if err := img.Resize(scale, options); err != nil {
		return nil, fmt.Errorf("failed to resize by height: %w", err)
	}
	return img, nil
}

func (o *ResizeOperation) resizeWidthOnly(img *vips.Image, width int) (*vips.Image, error) {
	currentWidth := img.Width()
	if currentWidth <= 0 {
		return nil, fmt.Errorf("failed to resize by width: invalid source width %d", currentWidth)
	}
	if width == currentWidth {
		return img, nil
	}

	scale := float64(width) / float64(currentWidth)
	options := vips.DefaultResizeOptions()
	options.Vscale = scale
	if err := img.Resize(scale, options); err != nil {
		return nil, fmt.Errorf("failed to resize by width: %w", err)
	}
	return img, nil
}

func (o *ResizeOperation) resizeBoth(img *vips.Image, width, height int) (*vips.Image, error) {
	if o.Fit == "fill" {
		return o.resizeFill(img, width, height)
	}
	// Default to cover mode
	err := img.ThumbnailImage(width, &vips.ThumbnailImageOptions{
		Height: height,
		Crop:   vips.InterestingCentre,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resize (cover): %w", err)
	}
	return img, nil
}

func (o *ResizeOperation) resizeFill(img *vips.Image, targetWidth, targetHeight int) (*vips.Image, error) {
	// Resize to fit within target dimensions (maintaining aspect ratio)
	err := img.ThumbnailImage(targetWidth, &vips.ThumbnailImageOptions{
		Height: targetHeight,
		Size:   vips.SizeDown,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resize for fill mode: %w", err)
	}

	// Get new dimensions after resize
	newWidth := img.Width()
	newHeight := img.Height()

	// If already matches target size, return
	if newWidth == targetWidth && newHeight == targetHeight {
		return img, nil
	}

	// Calculate padding needed to center the image
	left := (targetWidth - newWidth) / 2
	top := (targetHeight - newHeight) / 2

	// Handle transparent mode - only works with PNG format
	if o.FillColor == "transparent" {
		// Use transparent background (RGBA)
		err = img.Embed(left, top, targetWidth, targetHeight, &vips.EmbedOptions{
			Extend:     vips.ExtendBackground,
			Background: []float64{0, 0, 0, 0},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to embed image with transparent background: %w", err)
		}
		return img, nil
	}

	// Determine background color for non-transparent fills
	var bgColor []float64
	switch o.FillColor {
	case "black":
		bgColor = []float64{0, 0, 0}
	default: // "white"
		bgColor = []float64{255, 255, 255}
	}

	// Embed the image in a canvas with the target dimensions
	err = img.Embed(left, top, targetWidth, targetHeight, &vips.EmbedOptions{
		Extend:     vips.ExtendBackground,
		Background: bgColor,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to embed image: %w", err)
	}

	return img, nil
}

func parsePositiveInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	num := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("must be a positive integer, got: %s", s)
		}
		num = num*10 + int(ch-'0')
	}

	if num <= 0 {
		return 0, fmt.Errorf("must be positive, got: %d", num)
	}

	return num, nil
}
