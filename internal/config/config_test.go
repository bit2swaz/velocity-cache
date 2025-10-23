package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()

	data, err := os.ReadFile("testdata/test.config.json")
	require.NoError(t, err, "read fixture")

	configPath := filepath.Join(tmpDir, "velocity.config.json")
	require.NoError(t, os.WriteFile(configPath, data, 0o644), "write config")

	wd, err := os.Getwd()
	require.NoError(t, err, "get working directory")
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd), "restore working directory")
	})

	require.NoError(t, os.Chdir(tmpDir), "chdir to temp directory")

	cfg, err := Load()
	require.NoError(t, err, "Load should not return an error")

	assert.True(t, cfg.RemoteCache.Enabled, "RemoteCache.Enabled expected true")
	assert.Equal(t, "velocity-cache-mvp-public", cfg.RemoteCache.Bucket)
	assert.Equal(t, "us-east-1", cfg.RemoteCache.Region)

	script, ok := cfg.Scripts["build"]
	require.True(t, ok, "Scripts[\"build\"] missing")

	assert.Equal(t, "npm run build", script.Command)
	assert.Len(t, script.Inputs, 6, "expected 6 inputs")
	assert.Equal(t, []string{".next/"}, script.Outputs)
	assert.Equal(t, []string{"NODE_ENV"}, script.EnvKeys)
}
