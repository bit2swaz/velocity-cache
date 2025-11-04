package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bit2swaz/velocity-cache/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvHashing(t *testing.T) {
	t.Setenv("NODE_ENV", "")
	cfg := config.TaskConfig{
		Command: "npm run build",
		EnvKeys: []string{"NODE_ENV"},
	}

	hash1, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "first hash should succeed")
	hash2, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "second hash should succeed")
	assert.Equal(t, hash1, hash2, "expected deterministic hash")

	t.Setenv("NODE_ENV", "production")
	hash3, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "hash with env change should succeed")
	assert.NotEqual(t, hash1, hash3, "expected env change to alter hash")
}

func TestCommandHashing(t *testing.T) {
	cfg := config.TaskConfig{
		Command: "npm run build",
	}

	hash1, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "first hash should succeed")
	hash2, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "second hash should succeed")
	assert.Equal(t, hash1, hash2, "expected deterministic hash")

	cfg.Command = "npm run test"
	hash3, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "hash after command change should succeed")
	assert.NotEqual(t, hash1, hash3, "expected command change to alter hash")
}

func TestFileInputsAffectHash(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0o644))

	wd, err := os.Getwd()
	require.NoError(t, err, "get working directory")
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd), "restore working directory")
	})

	require.NoError(t, os.Chdir(tmpDir), "change to temp directory")

	cfg := config.TaskConfig{
		Command: "npm run build",
		Inputs:  []string{"*.txt"},
	}

	hash1, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "hash with initial files should succeed")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("c"), 0o644))

	hash2, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "hash after adding file should succeed")
	assert.NotEqual(t, hash1, hash2, "adding matching file should alter hash")
}

func TestGitignoreFiltersFiles(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("ignored.txt\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "included.txt"), []byte("include"), 0o644))

	wd, err := os.Getwd()
	require.NoError(t, err, "get working directory")
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd), "restore working directory")
	})

	require.NoError(t, os.Chdir(tmpDir), "change to temp directory")

	cfg := config.TaskConfig{
		Command: "npm run build",
		Inputs:  []string{"*.txt"},
	}

	hash1, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "hash with ignored file absent should succeed")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ignored.txt"), []byte("ignore"), 0o644))

	hash2, err := GenerateCacheKey(cfg)
	require.NoError(t, err, "hash with ignored file should succeed")
	assert.Equal(t, hash1, hash2, "ignored file should not alter hash")
}

func TestGenerateCacheKeyIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	write := func(rel, contents string) {
		path := filepath.Join(tmpDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	write(".gitignore", "ignored.txt\nignored-dir/\n")
	write("src/a.txt", "alpha")
	write("src/b.txt", "beta")
	write("ignored.txt", "ignored contents")
	write("ignored-dir/c.txt", "ignored directory file")

	wd, err := os.Getwd()
	require.NoError(t, err, "get working directory")
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd), "restore working directory")
	})

	require.NoError(t, os.Chdir(tmpDir), "change to temp directory")

	cfg := config.TaskConfig{
		Command: "npm run build",
		Inputs:  []string{"**/*.txt"},
	}

	run := func() string {
		t.Helper()
		hash, err := GenerateCacheKey(cfg)
		require.NoError(t, err, "GenerateCacheKey should succeed")
		return hash
	}

	hash1 := run()

	hash2 := run()
	assert.Equal(t, hash1, hash2, "determinism check should pass")

	write("src/a.txt", "alpha-modified")
	hash3 := run()
	assert.NotEqual(t, hash1, hash3, "changing tracked file should alter hash")

	write("src/a.txt", "alpha")
	hash4Before := run()
	assert.Equal(t, hash1, hash4Before, "restoring tracked file should restore hash")

	write("ignored.txt", "updated ignored contents")
	hash4 := run()
	assert.Equal(t, hash1, hash4, "ignored file change should not alter hash")

	write("src/new.txt", "new tracked file")
	hash5 := run()
	assert.NotEqual(t, hash1, hash5, "adding tracked file should alter hash")
}
