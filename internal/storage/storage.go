package storage

import (
	"context"
)

// Storage interface for different storage backends
type Storage interface {
	GetObject(ctx context.Context, key string) ([]byte, error)
}
