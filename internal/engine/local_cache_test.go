package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLocalMissing(t *testing.T) {
	withTempWorkdir(t, func(root string) {
		path, found, err := checkLocal("abc123")
		if err != nil {
			t.Fatalf("checkLocal unexpected error: %v", err)
		}
		expectedPath := filepath.Join(root, ".velocity", "cache", "abc123.zip")
		if path != expectedPath {
			t.Fatalf("unexpected path: got %s want %s", path, expectedPath)
		}
		if found {
			t.Fatalf("expected found=false")
		}
	})
}

func TestLocalCacheIntegration(t *testing.T) {
	withTempWorkdir(t, func(root string) {
		srcZip := filepath.Join(root, "source.zip")
		if err := os.WriteFile(srcZip, []byte("zipdata"), 0o644); err != nil {
			t.Fatalf("write source zip: %v", err)
		}

		dest, err := saveLocal("key", srcZip)
		if err != nil {
			t.Fatalf("saveLocal error: %v", err)
		}

		expectedDest := filepath.Join(root, ".velocity", "cache", "key.zip")
		if dest != expectedDest {
			t.Fatalf("unexpected dest: got %s want %s", dest, expectedDest)
		}

		data, err := os.ReadFile(expectedDest)
		if err != nil {
			t.Fatalf("read cached zip: %v", err)
		}
		if string(data) != "zipdata" {
			t.Fatalf("unexpected data: got %q", string(data))
		}

		path, found, err := checkLocal("key")
		if err != nil {
			t.Fatalf("checkLocal err: %v", err)
		}
		if !found {
			t.Fatalf("expected cache to be found")
		}
		if path != expectedDest {
			t.Fatalf("unexpected path: got %s want %s", path, expectedDest)
		}

		if err := cleanLocal(); err != nil {
			t.Fatalf("cleanLocal error: %v", err)
		}

		cacheDir := filepath.Join(root, ".velocity", "cache")
		if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
			t.Fatalf("expected cache dir removed, got %v", err)
		}
	})
}

func TestSaveLocalOverwrite(t *testing.T) {
	withTempWorkdir(t, func(root string) {
		src := filepath.Join(root, "source.zip")
		if err := os.WriteFile(src, []byte("first"), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
		if _, err := saveLocal("cache", src); err != nil {
			t.Fatalf("saveLocal first: %v", err)
		}

		if err := os.WriteFile(src, []byte("second"), 0o644); err != nil {
			t.Fatalf("write source second: %v", err)
		}
		if _, err := saveLocal("cache", src); err != nil {
			t.Fatalf("saveLocal second: %v", err)
		}

		cached := filepath.Join(root, ".velocity", "cache", "cache.zip")
		data, err := os.ReadFile(cached)
		if err != nil {
			t.Fatalf("read cached: %v", err)
		}
		if string(data) != "second" {
			t.Fatalf("expected overwrite to update data, got %q", string(data))
		}
	})
}

func TestCleanLocal(t *testing.T) {
	withTempWorkdir(t, func(root string) {
		cacheDir := filepath.Join(root, ".velocity", "cache")
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			t.Fatalf("mkdir cache: %v", err)
		}

		file := filepath.Join(cacheDir, "foo.zip")
		if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
			t.Fatalf("write cache file: %v", err)
		}

		if err := cleanLocal(); err != nil {
			t.Fatalf("cleanLocal error: %v", err)
		}

		if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
			t.Fatalf("expected cache dir removed, got %v", err)
		}
	})
}

func TestCheckLocalInvalidKey(t *testing.T) {
	if _, _, err := checkLocal("../bad"); err == nil {
		t.Fatalf("expected error for invalid key")
	}
}

func withTempWorkdir(t *testing.T, fn func(root string)) {
	t.Helper()

	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	fn(tempDir)
}
