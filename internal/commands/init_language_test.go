package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

func TestInitDetectsPythonProject(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("pytest==7.0.0"), 0o644))

	runLanguageInitTest(t, tmpDir, func(cfg config.Config, output string) {
		assert.Contains(t, cfg.Pipeline, "test")
		assert.Contains(t, cfg.Pipeline, "lint")
		assert.Equal(t, "pytest", cfg.Pipeline["test"].Command)
		assert.Equal(t, "flake8", cfg.Pipeline["lint"].Command)
		assert.Equal(t, []string{"**/*.py", "requirements.txt", "poetry.lock"}, cfg.Pipeline["test"].Inputs)
		assert.Equal(t, []string{".venv/", ".cache/", "__pycache__/"}, cfg.Pipeline["test"].Outputs)
		assert.Contains(t, output, "Generated velocity.yml")
	})
}

func TestInitDetectsRustProject(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"demo\""), 0o644))

	runLanguageInitTest(t, tmpDir, func(cfg config.Config, output string) {
		assert.Contains(t, cfg.Pipeline, "build")
		assert.Contains(t, cfg.Pipeline, "test")
		assert.Equal(t, "cargo build --release", cfg.Pipeline["build"].Command)
		assert.Equal(t, "cargo test", cfg.Pipeline["test"].Command)
		assert.Equal(t, []string{"src/**/*.rs", "Cargo.toml", "Cargo.lock"}, cfg.Pipeline["build"].Inputs)
		assert.Equal(t, []string{"target/"}, cfg.Pipeline["build"].Outputs)
		assert.Contains(t, output, "Generated velocity.yml")
	})
}

func TestInitDetectsGoProject(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/demo"), 0o644))

	runLanguageInitTest(t, tmpDir, func(cfg config.Config, output string) {
		assert.Contains(t, cfg.Pipeline, "build")
		assert.Equal(t, "go build -o bin/demo ./cmd/...", cfg.Pipeline["build"].Command)
		assert.Equal(t, []string{"**/*.go", "go.mod", "go.sum"}, cfg.Pipeline["build"].Inputs)
		assert.Equal(t, []string{"bin/"}, cfg.Pipeline["build"].Outputs)
		assert.Contains(t, output, "Generated velocity.yml")
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

	data, err := os.ReadFile(filepath.Join(dir, "velocity.yml"))
	require.NoError(t, err)

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assertFn(cfg, stdout.String())
}
