package engine

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressExtractRoundTrip(t *testing.T) {
	tempDir := t.TempDir()

	alpha := filepath.Join(tempDir, "alpha")
	mustMkdirAll(t, alpha)
	mustWriteFile(t, filepath.Join(alpha, "file1.txt"), "alpha-file")
	mustMkdirAll(t, filepath.Join(alpha, "nested"))
	mustWriteFile(t, filepath.Join(alpha, "nested", "file2.txt"), "nested")
	mustMkdirAll(t, filepath.Join(alpha, "empty"))

	beta := filepath.Join(tempDir, "beta")
	mustMkdirAll(t, beta)
	mustWriteFile(t, filepath.Join(beta, "b1.txt"), "beta")
	mustMkdirAll(t, filepath.Join(beta, "inner", "deep"))
	mustWriteFile(t, filepath.Join(beta, "inner", "deep", "b2.txt"), "deep")

	archivePath := filepath.Join(tempDir, "artifact.zip")

	if err := compress([]string{alpha, beta}, archivePath, ""); err != nil {
		t.Fatalf("compress returned error: %v", err)
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("expected archive to exist: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected archive to be a file, got mode %v", info.Mode())
	}
	if info.Size() == 0 {
		t.Fatalf("expected archive %s to be non-empty", archivePath)
	}

	junkFile := filepath.Join(alpha, "junk.txt")
	mustWriteFile(t, junkFile, "junk")
	junkDir := filepath.Join(beta, "junkdir")
	mustMkdirAll(t, junkDir)
	mustWriteFile(t, filepath.Join(junkDir, "junk"), "junk")

	if err := extract(archivePath, []string{alpha, beta}, ""); err != nil {
		t.Fatalf("extract returned error: %v", err)
	}

	if _, err := os.Stat(junkFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected junk file to be removed, got %v", err)
	}
	if _, err := os.Stat(junkDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected junk directory to be removed, got %v", err)
	}

	assertFileContent(t, filepath.Join(alpha, "file1.txt"), "alpha-file")
	assertFileContent(t, filepath.Join(alpha, "nested", "file2.txt"), "nested")
	assertDirExists(t, filepath.Join(alpha, "empty"))

	assertFileContent(t, filepath.Join(beta, "b1.txt"), "beta")
	assertFileContent(t, filepath.Join(beta, "inner", "deep", "b2.txt"), "deep")
}

func TestCompressDuplicateBaseName(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "dup")
	second := filepath.Join(tempDir, "nested", "dup")
	mustMkdirAll(t, first)
	mustMkdirAll(t, second)

	err := compress([]string{first, second}, filepath.Join(tempDir, "dup.zip"), "")
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate base name error, got %v", err)
	}
}

func TestCompressMissingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	err := compress([]string{filepath.Join(tempDir, "missing")}, filepath.Join(tempDir, "missing.zip"), "")
	if err == nil || !strings.Contains(err.Error(), "stat") {
		t.Fatalf("expected stat error for missing directory, got %v", err)
	}
}

func TestExtractUnexpectedRoot(t *testing.T) {
	tempDir := t.TempDir()
	archive := filepath.Join(tempDir, "bad.zip")

	createZip(t, archive, map[string]string{"other/file.txt": "data"})

	target := filepath.Join(tempDir, "alpha")
	mustMkdirAll(t, target)

	err := extract(archive, []string{target}, "")
	if err == nil || !strings.Contains(err.Error(), "unexpected archive root") {
		t.Fatalf("expected unexpected archive root error, got %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("unexpected file content for %s: got %q want %q", path, string(data), expected)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat dir %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", path)
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	archive, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip %s: %v", path, err)
	}
	defer archive.Close()

	writer := zip.NewWriter(archive)
	defer writer.Close()

	for name, contents := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
}
