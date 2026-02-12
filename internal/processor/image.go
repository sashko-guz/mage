package processor

import (
	"fmt"

	"github.com/cshum/vipsgen/vips"
)

type ImageProcessor struct{}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{}
}

type ThumbnailOptions struct {
	Width   int
	Height  int
	Format  string
	Quality int
	Fit     string // "fill", or "cover"
}

func (p *ImageProcessor) CreateThumbnail(imageData []byte, opts *ThumbnailOptions) ([]byte, string, error) {
	var img *vips.Image
	var err error

	// Choose processing based on fit mode
	switch opts.Fit {
	case "fill":
		// Fit within bounds, maintaining aspect ratio
		img, err = vips.NewThumbnailBuffer(imageData, opts.Width, &vips.ThumbnailBufferOptions{
			Height: opts.Height,
			Size:   vips.SizeBoth,
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to create thumbnail: %w", err)
		}
		
		// If image is smaller than requested dimensions, extend it to fill
		imgWidth := img.Width()
		imgHeight := img.Height()
		if imgWidth < opts.Width || imgHeight < opts.Height {
			// Use Gravity to center and extend to exact dimensions
			err = img.Gravity(vips.CompassDirectionCentre, opts.Width, opts.Height, &vips.GravityOptions{
				Extend: vips.ExtendBlack,
			})
			if err != nil {
				img.Close()
				return nil, "", fmt.Errorf("failed to extend image: %w", err)
			}
		}
	default:
		// Default to cover mode 
		img, err = vips.NewThumbnailBuffer(imageData, opts.Width, &vips.ThumbnailBufferOptions{
			Height: opts.Height,
			Crop:   vips.InterestingCentre,
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to create thumbnail: %w", err)
		}
	}
	defer img.Close()

	// Export based on format
	var result []byte
	var contentType string

	switch opts.Format {
	case "webp":
		result, err = img.WebpsaveBuffer(&vips.WebpsaveBufferOptions{
			Q: opts.Quality,
		})
		contentType = "image/webp"
	case "png":
		result, err = img.PngsaveBuffer(&vips.PngsaveBufferOptions{
			Q: opts.Quality,
		})
		contentType = "image/png"
	case "jpeg", "jpg":
		result, err = img.JpegsaveBuffer(&vips.JpegsaveBufferOptions{
			Q: opts.Quality,
		})
		contentType = "image/jpeg"
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", opts.Format)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to export image: %w", err)
	}

	return result, contentType, nil
}
