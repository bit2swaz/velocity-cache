package storage

import "context"

// Driver defines the interface for storage backends.
type Driver interface {
	GetUploadURL(ctx context.Context, key string) (string, error)
	GetDownloadURL(ctx context.Context, key string) (string, error)
	Exists(ctx context.Context, key string) (bool, error)
}
