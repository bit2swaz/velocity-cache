package storage

import "context"

type Driver interface {
	GetUploadURL(ctx context.Context, key string) (string, error)
	GetDownloadURL(ctx context.Context, key string) (string, error)
	Exists(ctx context.Context, key string) (bool, error)
}
