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
func NewRegistry() *Registry {
	r := &Registry{
		resizeOp:  NewResizeOperation(),
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

// ApplyAll applies all operations in the request
func (r *Registry) ApplyAll(imageData []byte, req *Request) ([]byte, string, error) {
	// Load image with EXIF-based autorotation to avoid rotated outputs.
	loadOptions := vips.DefaultLoadOptions()
	loadOptions.Autorotate = true
	img, err := vips.NewImageFromBuffer(imageData, loadOptions)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load image: %w", err)
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
		formatOp = r.formatOp // Fallback to default (shouldn't happen)
	}
	if qualityOp == nil {
		qualityOp = r.qualityOp // Fallback to default (shouldn't happen)
	}

	return formatOp.Export(img, qualityOp.Quality)
}
