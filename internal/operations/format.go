package operations

import (
	"fmt"
	"strings"

	"github.com/cshum/vipsgen/vips"
)

// FormatOperation handles format(type) filter
type FormatOperation struct {
	Format string
}

func NewFormatOperation() *FormatOperation {
	return &FormatOperation{
		Format: "jpeg", // Default format
	}
}

func (o *FormatOperation) Name() string {
	return "format"
}

func (o *FormatOperation) Clone() Operation {
	return NewFormatOperation()
}

func (o *FormatOperation) Parse(filter string) (bool, error) {
	if !strings.HasPrefix(filter, "format(") {
		return false, nil
	}

	if !strings.HasSuffix(filter, ")") {
		return false, fmt.Errorf("format filter missing closing parenthesis")
	}

	content := filter[7 : len(filter)-1]
	content = strings.TrimSpace(content)

	if content == "" {
		return false, fmt.Errorf("format filter requires a format type")
	}

	format := strings.ToLower(content)
	switch format {
	case "webp", "jpeg", "png", "jpg":
		o.Format = format
		return true, nil
	default:
		return false, fmt.Errorf("unsupported format: %s (supported: webp, jpeg, png)", format)
	}
}

func (o *FormatOperation) Apply(img *vips.Image) (*vips.Image, error) {
	// Format is applied during export
	return img, nil
}

// Export exports the image to the requested format
func (o *FormatOperation) Export(img *vips.Image, quality int) ([]byte, string, error) {
	var result []byte
	var contentType string
	var err error

	switch o.Format {
	case "webp":
		result, err = img.WebpsaveBuffer(&vips.WebpsaveBufferOptions{
			Q: quality,
		})
		contentType = "image/webp"
	case "png":
		result, err = img.PngsaveBuffer(&vips.PngsaveBufferOptions{
			Q: quality,
		})
		contentType = "image/png"
	case "jpeg", "jpg":
		result, err = img.JpegsaveBuffer(&vips.JpegsaveBufferOptions{
			Q: quality,
		})
		contentType = "image/jpeg"
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", o.Format)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to export image: %w", err)
	}

	return result, contentType, nil
}

// DetectFromExtension sets format based on file extension
func (o *FormatOperation) DetectFromExtension(path string) {
	ext := strings.ToLower(path)
	if strings.HasSuffix(ext, ".webp") {
		o.Format = "webp"
	} else if strings.HasSuffix(ext, ".png") {
		o.Format = "png"
	} else if strings.HasSuffix(ext, ".jpg") {
		o.Format = "jpeg"
	} else if strings.HasSuffix(ext, ".jpeg") {
		o.Format = "jpeg"
	}
}
