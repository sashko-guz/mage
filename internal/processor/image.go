package processor

import (
	"github.com/sashko-guz/mage/internal/operations"
)

type ImageProcessor struct{}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{}
}

func (p *ImageProcessor) CreateThumbnail(imageData []byte, req *operations.Request) ([]byte, string, error) {
	return operations.ApplyAll(imageData, req)
}
