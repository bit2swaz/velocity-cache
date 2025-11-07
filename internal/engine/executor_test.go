package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bit2swaz/velocity-cache/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	script := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("stdout message")
	_, _ = os.Stderr.WriteString("stderr message\n")
}
`

	progPath := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(progPath, []byte(script), 0o644))

	cfg := config.TaskConfig{Command: "go run " + progPath}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code, err := executeWithWriters(cfg, tmpDir, &stdout, &stderr)
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "stdout message")
	assert.Contains(t, stderr.String(), "stderr message")
}

func TestExecuteFailure(t *testing.T) {
	cfg := config.TaskConfig{Command: "sh -c 'echo fail >&2; exit 1'"}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code, err := executeWithWriters(cfg, t.TempDir(), &stdout, &stderr)
	assert.Error(t, err)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr.String(), "fail")
}
