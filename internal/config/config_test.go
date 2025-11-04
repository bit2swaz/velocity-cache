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

	assert.Equal(t, []string{"packages/app", "packages/api"}, cfg.Packages)
	assert.Len(t, cfg.Tasks, 3, "expected three tasks")

	buildTask, ok := cfg.Tasks["build"]
	require.True(t, ok, "Tasks[\"build\"] missing")
	assert.Equal(t, "npm run build", buildTask.Command)
	assert.Equal(t, []string{"lint"}, buildTask.DependsOn)
	assert.Len(t, buildTask.Inputs, 7, "expected 7 inputs")
	assert.Equal(t, []string{".next/"}, buildTask.Outputs)
	assert.Equal(t, []string{"NODE_ENV"}, buildTask.EnvKeys)

	lintTask, ok := cfg.Tasks["lint"]
	require.True(t, ok, "Tasks[\"lint\"] missing")
	assert.Equal(t, "npm run lint", lintTask.Command)
	assert.Equal(t, []string{"prepare"}, lintTask.DependsOn)
	assert.Equal(t, []string{"src/**/*", "packages/app/**/*", "packages/api/**/*"}, lintTask.Inputs)
	assert.Empty(t, lintTask.Outputs)
	assert.Empty(t, lintTask.EnvKeys)

	prepareTask, ok := cfg.Tasks["prepare"]
	require.True(t, ok, "Tasks[\"prepare\"] missing")
	assert.Equal(t, "npm install", prepareTask.Command)
	assert.Empty(t, prepareTask.DependsOn)
	assert.Equal(t, []string{"package.json", "package-lock.json"}, prepareTask.Inputs)
	assert.Equal(t, []string{"node_modules/"}, prepareTask.Outputs)
	assert.Empty(t, prepareTask.EnvKeys)
}
