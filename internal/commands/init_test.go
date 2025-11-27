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

func TestInitGeneratesVelocityConfigFromTurbo(t *testing.T) {
	tmpDir := t.TempDir()

	write := func(rel, contents string) {
		t.Helper()
		path := filepath.Join(tmpDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	write("package.json", `{
  "workspaces": [
    "apps/*",
    "packages/*"
  ]
}`)

	write("turbo.json", `{
  "pipeline": {
    "build": {
      "dependsOn": ["^build"],
      "inputs": ["packages/app/**"],
      "outputs": ["dist/app"],
      "env": ["NODE_ENV"]
    },
    "lint": {
      "dependsOn": [],
      "inputs": ["packages/**/*.ts"],
      "outputs": []
    }
  }
}`)

	wd, err := os.Getwd()
	require.NoError(t, err, "get working directory")
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd), "restore working directory")
	})

	require.NoError(t, os.Chdir(tmpDir), "change to temp directory")

	cmd := newInitCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)

	require.NoError(t, runInit(cmd), "runInit should succeed")

	generatedPath := filepath.Join(tmpDir, "velocity.yml")
	data, err := os.ReadFile(generatedPath)
	require.NoError(t, err, "read generated config")

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg), "parse generated config")

	assert.Equal(t, []string{"apps/*", "packages/*"}, cfg.Packages, "packages should reflect workspaces")

	require.Contains(t, cfg.Pipeline, "build", "build task should be present")
	require.Contains(t, cfg.Pipeline, "lint", "lint task should be present")

	buildTask := cfg.Pipeline["build"]
	assert.Equal(t, "npm run build", buildTask.Command)
	assert.Equal(t, []string{"^build"}, buildTask.DependsOn)
	assert.Equal(t, []string{"packages/app/**"}, buildTask.Inputs)
	assert.Equal(t, []string{"dist/app"}, buildTask.Outputs)
	assert.Equal(t, []string{"NODE_ENV"}, buildTask.EnvKeys)

	lintTask := cfg.Pipeline["lint"]
	assert.Equal(t, "npm run lint", lintTask.Command)
	assert.Empty(t, lintTask.DependsOn)
	assert.Equal(t, []string{"packages/**/*.ts"}, lintTask.Inputs)
	assert.Empty(t, lintTask.Outputs)
	assert.Empty(t, lintTask.EnvKeys)

	expectedMessage := "Generated velocity.yml\n"
	assert.Equal(t, expectedMessage, stdout.String(), "init command should report generation")
}
