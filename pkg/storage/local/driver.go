package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalDriver struct {
	root    string
	baseURL string
}

func New() (*LocalDriver, error) {
	root := os.Getenv("VC_LOCAL_ROOT")
	if root == "" {
		return nil, fmt.Errorf("VC_LOCAL_ROOT is not set")
	}

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

func (d *LocalDriver) GetUploadURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/v1/proxy/blob/%s", d.baseURL, key), nil
}

func (d *LocalDriver) GetDownloadURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/v1/proxy/blob/%s", d.baseURL, key), nil
}

func (d *LocalDriver) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(d.root, key)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
