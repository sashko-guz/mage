package processor

import (
	"github.com/sashko-guz/mage/internal/operations"
)

type ImageProcessor struct {
	registry *operations.Registry
}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		registry: operations.NewRegistry(),
	}
}

func (p *ImageProcessor) CreateThumbnail(imageData []byte, req *operations.Request) ([]byte, string, error) {
	// Use the operations registry to process the image
	// Operations are applied in the order they were parsed
	return p.registry.ApplyAll(imageData, req)
}
