package operations

import (
	"fmt"

	"github.com/cshum/vipsgen/vips"
)

// Registry manages all available operations
type Registry struct {
	prototypes []Operation // Prototype operations for cloning
	resizeOp   *ResizeOperation
	formatOp   *FormatOperation
	qualityOp  *QualityOperation
}

// NewRegistry creates a new operation registry with all operations registered
func NewRegistry(maxWidth, maxHeight, maxResolution int) *Registry {
	r := &Registry{
		resizeOp:  NewResizeOperation(maxWidth, maxHeight, maxResolution),
		formatOp:  NewFormatOperation(),
		qualityOp: NewQualityOperation(),
	}

	// Register filter operation prototypes
	r.prototypes = []Operation{
		r.formatOp,
		r.qualityOp,
		NewFitOperation(),
		NewCropOperation(),
		NewPercentCropOperation(),
	}

	return r
}

// ParseFilter attempts to parse a filter using registered operations
// Returns a new operation instance if parsing succeeds
func (r *Registry) ParseFilter(filter string) (Operation, error) {
	for _, prototype := range r.prototypes {
		op := prototype.Clone()
		handled, err := op.Parse(filter)
		if err != nil {
			return nil, fmt.Errorf("operation %s: %w", op.Name(), err)
		}
		if handled {
			return op, nil
		}
	}
	return nil, fmt.Errorf("unknown filter: %s", filter)
}

// ResizeOp returns the resize operation for size parsing
func (r *Registry) ResizeOp() *ResizeOperation {
	return r.resizeOp
}

// FormatOp returns the format operation
func (r *Registry) FormatOp() *FormatOperation {
	return r.formatOp
}

// QualityOp returns the quality operation
func (r *Registry) QualityOp() *QualityOperation {
	return r.qualityOp
}

func prepareImage(imageData []byte) (*vips.Image, error) {
	// Load image, then apply EXIF-based autorotation.
	// Autorotate cannot be set in load options because not all loaders support it (e.g. WebP).
	img, err := vips.NewImageFromBuffer(imageData, vips.DefaultLoadOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %w", err)
	}

	if err := img.Autorot(&vips.AutorotOptions{}); err != nil {
		img.Close()
		return nil, fmt.Errorf("failed to autorotate image: %w", err)
	}

	return img, nil
}

// ApplyAll applies all operations in the request to the image data
func ApplyAll(imageData []byte, req *Request) ([]byte, string, error) {
	img, err := prepareImage(imageData)
	if err != nil {
		return nil, "", err
	}
	defer img.Close()

	// Extract special operations
	var formatOp *FormatOperation
	var qualityOp *QualityOperation
	var resizeOp *ResizeOperation

	// Separate resize from other processing operations
	var processingOps []Operation

	for _, op := range req.Operations {
		switch v := op.(type) {
		case *FormatOperation:
			formatOp = v
		case *QualityOperation:
			qualityOp = v
		case *ResizeOperation:
			resizeOp = v
		default:
			processingOps = append(processingOps, op)
		}
	}

	// Apply processing operations first (crop, fit, etc.)
	for _, op := range processingOps {
		img, err = op.Apply(img)
		if err != nil {
			return nil, "", fmt.Errorf("operation %s failed: %w", op.Name(), err)
		}
	}

	// Apply resize LAST to ensure output dimensions match request
	if resizeOp != nil {
		img, err = resizeOp.Apply(img)
		if err != nil {
			return nil, "", fmt.Errorf("operation %s failed: %w", resizeOp.Name(), err)
		}
	}

	// Use extracted format and quality for export
	if formatOp == nil {
		formatOp = NewFormatOperation()
	}
	if qualityOp == nil {
		qualityOp = NewQualityOperation()
	}

	return formatOp.Export(img, qualityOp.Quality)
}
