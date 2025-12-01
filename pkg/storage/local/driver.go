package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalDriver implements storage.Driver for local filesystem storage.
type LocalDriver struct {
	root    string
	baseURL string
}

// New creates a new LocalDriver.
func New() (*LocalDriver, error) {
	root := os.Getenv("VC_LOCAL_ROOT")
	if root == "" {
		return nil, fmt.Errorf("VC_LOCAL_ROOT is not set")
	}

	// Default to localhost:8080 if not set, but allow override
	baseURL := os.Getenv("VC_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("failed to create local root directory: %w", err)
	}

	return &LocalDriver{root: root, baseURL: baseURL}, nil
}

// GetUploadURL returns the URL for uploading a file.
func (d *LocalDriver) GetUploadURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/v1/proxy/blob/%s", d.baseURL, key), nil
}

// GetDownloadURL returns the URL for downloading a file.
func (d *LocalDriver) GetDownloadURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/v1/proxy/blob/%s", d.baseURL, key), nil
}

// Exists checks if the file exists in the local filesystem.
func (d *LocalDriver) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(d.root, key)
	_, err := os.Stat(path)
	if err == nil {
		// UPDATE: Touch the file to reset its eviction timer
		// We ignore errors here because it's an optimization, not critical
		now := time.Now()
		os.Chtimes(path, now, now)
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
