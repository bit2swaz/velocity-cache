package engine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

// Execute runs the script command defined in cfg and streams output directly to the caller's terminal.
// It returns the command's exit code along with any execution error.
func Execute(cfg config.TaskConfig) (int, error) {
	return executeWithWriters(cfg, os.Stdout, os.Stderr)
}

func executeWithWriters(cfg config.TaskConfig, stdout, stderr io.Writer) (int, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return -1, errors.New("command is empty")
	}

	shell := defaultShell()
	cmd := exec.Command(shell[0], append(shell[1:], command)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode := exitCodeFromSys(exitErr.Sys())
			return exitCode, err
		}

		return -1, fmt.Errorf("execute command: %w", err)
	}

	return 0, nil
}

func defaultShell() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/C"}
	}
	return []string{"/bin/sh", "-c"}
}

func exitCodeFromSys(sys interface{}) int {
	status, ok := sys.(syscall.WaitStatus)
	if !ok {
		return -1
	}
	return status.ExitStatus()
}
