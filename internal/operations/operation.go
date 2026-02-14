package operations

import "github.com/cshum/vipsgen/vips"

// Request contains core thumbnail request information
type Request struct {
	Path              string // Image path in storage
	ProvidedSignature string // URL signature for validation
	FilterString      string // Raw filter string for signature validation
	RawURLPath        string // Raw path after /thumbs/ (without query params) for signature validation

	// Parsed operations to apply (in order)
	Operations []Operation
}

// GetSize returns width and height from the ResizeOperation if present
func (r *Request) GetSize() (width, height *int) {
	for _, op := range r.Operations {
		if resizeOp, ok := op.(*ResizeOperation); ok {
			return resizeOp.Width, resizeOp.Height
		}
	}
	return nil, nil
}

// Operation defines both parsing and image processing for a filter
type Operation interface {
	// Name returns the operation identifier
	Name() string

	// Parse attempts to parse the filter string
	// Returns true if this operation handled the filter
	// The operation should store parsed values internally
	Parse(filter string) (handled bool, err error)

	// Apply applies the operation to the image
	Apply(img *vips.Image) (*vips.Image, error)

	// Clone creates a new instance for parsing
	// This allows multiple instances of the same operation type
	Clone() Operation
}
