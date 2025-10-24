package engine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	velocityDirName = ".velocity"
	cacheDirName    = "cache"
	cacheFileExt    = ".zip"
	cacheMetaExt    = ".meta.json"
)

func checkLocal(cacheKey string) (string, bool, error) {
	if err := validateCacheKey(cacheKey); err != nil {
		return "", false, err
	}

	path, err := localCacheFile(cacheKey)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		return "", false, fmt.Errorf("check local cache stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("check local cache %s is not a regular file", path)
	}

	return path, true, nil
}

func saveLocal(cacheKey, zipPath string) (string, error) {
	if err := validateCacheKey(cacheKey); err != nil {
		return "", err
	}

	cleanedZip := filepath.Clean(zipPath)
	info, err := os.Stat(cleanedZip)
	if err != nil {
		return "", fmt.Errorf("save local cache stat %s: %w", cleanedZip, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("save local cache %s is not a regular file", cleanedZip)
	}

	cacheDir, err := localCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("save local cache ensure dir %s: %w", cacheDir, err)
	}

	destination, err := localCacheFile(cacheKey)
	if err != nil {
		return "", err
	}
	if sameFile(cleanedZip, destination) {
		return destination, nil
	}

	if err := copyFile(cleanedZip, destination); err != nil {
		return "", err
	}

	return destination, nil
}

func cleanLocal() error {
	dir, err := localCacheDir()
	if err != nil {
		return err
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clean local cache remove %s: %w", dir, err)
	}

	return nil
}

func localCacheDir() (string, error) {
	dir := filepath.Join(velocityDirName, cacheDirName)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve cache dir %s: %w", dir, err)
	}
	return abs, nil
}

func localCacheFile(cacheKey string) (string, error) {
	dir, err := localCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheKey+cacheFileExt), nil
}

func localCacheMetadata(cacheKey string) (string, error) {
	if err := validateCacheKey(cacheKey); err != nil {
		return "", err
	}
	dir, err := localCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheKey+cacheMetaExt), nil
}

func validateCacheKey(cacheKey string) error {
	trimmed := strings.TrimSpace(cacheKey)
	if trimmed == "" {
		return errors.New("cache key is empty")
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, string(os.PathSeparator)) {
		return fmt.Errorf("cache key contains path separator: %s", cacheKey)
	}
	if strings.Contains(trimmed, "..") {
		return fmt.Errorf("cache key contains invalid sequence: %s", cacheKey)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source cache %s: %w", src, err)
	}

	out, err := os.Create(dst)
	if err != nil {
		in.Close()
		return fmt.Errorf("create destination cache %s: %w", dst, err)
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	inCloseErr := in.Close()

	if copyErr != nil {
		return fmt.Errorf("copy cache from %s to %s: %w", src, dst, copyErr)
	}
	if inCloseErr != nil {
		return fmt.Errorf("close source cache %s: %w", src, inCloseErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close destination cache %s: %w", dst, closeErr)
	}

	return nil
}

func sameFile(a, b string) bool {
	if a == b {
		return true
	}

	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}

	return absA == absB
}

func CheckLocal(cacheKey string) (string, bool, error) {
	return checkLocal(cacheKey)
}

func SaveLocal(cacheKey, zipPath string) (string, error) {
	return saveLocal(cacheKey, zipPath)
}

func CleanLocal() error {
	return cleanLocal()
}

func LocalCacheMetadataPath(cacheKey string) (string, error) {
	return localCacheMetadata(cacheKey)
}

func CacheMetadataObjectName(cacheKey string) (string, error) {
	if err := validateCacheKey(cacheKey); err != nil {
		return "", err
	}
	return cacheKey + cacheMetaExt, nil
}
