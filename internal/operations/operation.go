package operations

import "github.com/cshum/vipsgen/vips"

// Request contains core thumbnail request information
type Request struct {
	Path              string // Image path in storage
	Alias             string // Optional output alias filename from /as/{alias}
	AliasExtension    string // Optional normalized alias extension (jpeg/png/webp/avif)
	HasAlias          bool   // True when URL contains /as/{alias}
	ProvidedSignature string // URL signature for validation
	SignaturePayload  string // Canonical payload to sign/verify: /{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
	FilterString      string // Raw filter string for signature validation
	RawURLPath        string // Raw path after /thumbs/ (without query params) for signature validation

	// Parsed operations to apply (in order)
	Operations []Operation
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

// Validatable is an optional interface for operations that need additional validation.
// It is checked by the parser after operations are assembled.
type Validatable interface {
	Validate() error
}
