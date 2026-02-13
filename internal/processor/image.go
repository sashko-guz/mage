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
	Width     *int   // nil means preserve aspect ratio or original size
	Height    *int   // nil means preserve aspect ratio or original size
	Format    string
	Quality   int
	Fit       string // "fill" or "cover"
	FillColor string // "black", "white", or "transparent" (for fill mode only)
	CropX1    *int   // First point X coordinate for crop
	CropY1    *int   // First point Y coordinate for crop
	CropX2    *int   // Second point X coordinate for crop
	CropY2    *int   // Second point Y coordinate for crop
}

func (p *ImageProcessor) CreateThumbnail(imageData []byte, opts *ThumbnailOptions) ([]byte, string, error) {
	var img *vips.Image
	var err error

	// Check if we need to apply crop first
	needsCrop := opts.CropX1 != nil && opts.CropY1 != nil && opts.CropX2 != nil && opts.CropY2 != nil

	if needsCrop {
		// Load image first to apply crop
		img, err = vips.NewImageFromBuffer(imageData, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load image: %w", err)
		}

		// Calculate crop dimensions
		cropLeft := *opts.CropX1
		cropTop := *opts.CropY1
		cropWidth := *opts.CropX2 - *opts.CropX1
		cropHeight := *opts.CropY2 - *opts.CropY1

		// Validate crop coordinates are within image bounds
		imgWidth := img.Width()
		imgHeight := img.Height()

		if cropLeft < 0 || cropTop < 0 || cropLeft >= imgWidth || cropTop >= imgHeight ||
			*opts.CropX2 > imgWidth || *opts.CropY2 > imgHeight {
			img.Close()
			return nil, "", fmt.Errorf("crop coordinates out of bounds: image is %dx%d, crop area is (%d,%d) to (%d,%d)",
				imgWidth, imgHeight, cropLeft, cropTop, *opts.CropX2, *opts.CropY2)
		}

		// Apply crop
		err = img.ExtractArea(cropLeft, cropTop, cropWidth, cropHeight)
		if err != nil {
			img.Close()
			return nil, "", fmt.Errorf("failed to crop image: %w", err)
		}

		// After cropping, if no resize is needed, skip to export
		if opts.Width == nil && opts.Height == nil {
			// No resize needed after crop, go to export
		} else {
			// Need to resize the cropped image
			// Save cropped image to buffer and reload with thumbnail
			cropData, err := img.JpegsaveBuffer(&vips.JpegsaveBufferOptions{Q: 95})
			img.Close()
			if err != nil {
				return nil, "", fmt.Errorf("failed to save cropped image: %w", err)
			}

			// Reload and process with resize
			img, err = p.resizeImage(cropData, opts)
			if err != nil {
				return nil, "", err
			}
		}
	} else {
		// No crop, proceed with normal resize flow
		img, err = p.resizeImage(imageData, opts)
		if err != nil {
			return nil, "", err
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

// resizeImage handles the resize/fit logic
func (p *ImageProcessor) resizeImage(imageData []byte, opts *ThumbnailOptions) (*vips.Image, error) {
	var img *vips.Image
	var err error

	// Handle different sizing scenarios
	if opts.Width == nil && opts.Height == nil {
		// Original size - load without resizing, just apply format conversion
		img, err = vips.NewImageFromBuffer(imageData, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to load image: %w", err)
		}
	} else if opts.Width == nil {
		// Height only - maintain aspect ratio based on height
		img, err = vips.NewThumbnailBuffer(imageData, *opts.Height, &vips.ThumbnailBufferOptions{
			Height: *opts.Height,
			Size:   vips.SizeDown, // Only shrink, maintain aspect ratio
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create thumbnail: %w", err)
		}
	} else if opts.Height == nil {
		// Width only - maintain aspect ratio based on width
		img, err = vips.NewThumbnailBuffer(imageData, *opts.Width, &vips.ThumbnailBufferOptions{
			Size: vips.SizeDown, // Only shrink, maintain aspect ratio
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create thumbnail: %w", err)
		}
	} else {
		// Both width and height specified - use fit mode
		switch opts.Fit {
		case "fill":
			// Fit within bounds, maintaining aspect ratio
			img, err = vips.NewThumbnailBuffer(imageData, *opts.Width, &vips.ThumbnailBufferOptions{
				Height: *opts.Height,
				Size:   vips.SizeBoth,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create thumbnail: %w", err)
			}

			// If image is smaller than requested dimensions, extend it to fill
			imgWidth := img.Width()
			imgHeight := img.Height()
			if imgWidth < *opts.Width || imgHeight < *opts.Height {
				fillColor := opts.FillColor

				// Validate: transparent fill is only supported for PNG
				if fillColor == "transparent" && opts.Format != "png" {
					img.Close()
					return nil, fmt.Errorf("transparent fill is only supported for PNG format, got: %s", opts.Format)
				}

				// For transparent fill, ensure alpha channel exists
				if fillColor == "transparent" && !img.HasAlpha() {
					err = img.Addalpha()
					if err != nil {
						img.Close()
						return nil, fmt.Errorf("failed to add alpha channel: %w", err)
					}
				}

				var extendMode vips.Extend

				// Map color to extend mode
				switch fillColor {
				case "black":
					extendMode = vips.ExtendBlack
				case "white":
					extendMode = vips.ExtendWhite
				case "transparent":
					// For transparent, use background which respects alpha
					extendMode = vips.ExtendBackground
				default:
					extendMode = vips.ExtendWhite
				}

				// Use Gravity to center and extend to exact dimensions
				err = img.Gravity(vips.CompassDirectionCentre, *opts.Width, *opts.Height, &vips.GravityOptions{
					Extend: extendMode,
				})
				if err != nil {
					img.Close()
					return nil, fmt.Errorf("failed to extend image: %w", err)
				}
			}
		default:
			// Default to cover mode
			img, err = vips.NewThumbnailBuffer(imageData, *opts.Width, &vips.ThumbnailBufferOptions{
				Height: *opts.Height,
				Crop:   vips.InterestingCentre,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create thumbnail: %w", err)
			}
		}
	}

	return img, nil
}
