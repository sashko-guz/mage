package operations

import "fmt"

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
// Supports both canonical names (e.g. "quality") and aliases (e.g. "q")
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


