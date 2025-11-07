package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

func TestInitDetectsPythonProject(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("pytest==7.0.0"), 0o644))

	runLanguageInitTest(t, tmpDir, func(cfg config.Config, output string) {
		assert.Contains(t, cfg.Tasks, "test")
		assert.Contains(t, cfg.Tasks, "lint")
		assert.Equal(t, "pytest", cfg.Tasks["test"].Command)
		assert.Equal(t, "flake8", cfg.Tasks["lint"].Command)
		assert.Equal(t, []string{"**/*.py", "requirements.txt", "poetry.lock"}, cfg.Tasks["test"].Inputs)
		assert.Equal(t, []string{".venv/", ".cache/", "__pycache__/"}, cfg.Tasks["test"].Outputs)
		assert.Contains(t, output, "Generated velocity.config.json")
	})
}

func TestInitDetectsRustProject(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"demo\""), 0o644))

	runLanguageInitTest(t, tmpDir, func(cfg config.Config, output string) {
		assert.Contains(t, cfg.Tasks, "build")
		assert.Contains(t, cfg.Tasks, "test")
		assert.Equal(t, "cargo build --release", cfg.Tasks["build"].Command)
		assert.Equal(t, "cargo test", cfg.Tasks["test"].Command)
		assert.Equal(t, []string{"src/**/*.rs", "Cargo.toml", "Cargo.lock"}, cfg.Tasks["build"].Inputs)
		assert.Equal(t, []string{"target/"}, cfg.Tasks["build"].Outputs)
		assert.Contains(t, output, "Generated velocity.config.json")
	})
}

func TestInitDetectsGoProject(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/demo"), 0o644))

	runLanguageInitTest(t, tmpDir, func(cfg config.Config, output string) {
		assert.Contains(t, cfg.Tasks, "build")
		assert.Equal(t, "go build -o bin/demo ./cmd/...", cfg.Tasks["build"].Command)
		assert.Equal(t, []string{"**/*.go", "go.mod", "go.sum"}, cfg.Tasks["build"].Inputs)
		assert.Equal(t, []string{"bin/"}, cfg.Tasks["build"].Outputs)
		assert.Contains(t, output, "Generated velocity.config.json")
	})
}

func runLanguageInitTest(t *testing.T, dir string, assertFn func(config.Config, string)) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd))
	})

	require.NoError(t, os.Chdir(dir))

	cmd := newInitCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)

	require.NoError(t, runInit(cmd), "runInit should succeed")

	data, err := os.ReadFile(filepath.Join(dir, configFileName))
	require.NoError(t, err)

	var cfg config.Config
	require.NoError(t, json.Unmarshal(data, &cfg))

	assertFn(cfg, stdout.String())
}
