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

func Execute(cfg config.TaskConfig, packagePath string) (int, error) {
	return executeWithWriters(cfg, packagePath, os.Stdout, os.Stderr)
}

func executeWithWriters(cfg config.TaskConfig, packagePath string, stdout, stderr io.Writer) (int, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return -1, errors.New("command is empty")
	}

	originalWd := ""
	if strings.TrimSpace(packagePath) != "" {
		wd, err := os.Getwd()
		if err != nil {
			return -1, fmt.Errorf("getwd: %w", err)
		}
		if err := os.Chdir(packagePath); err != nil {
			return -1, fmt.Errorf("chdir to %s: %w", packagePath, err)
		}
		originalWd = wd
		defer func() {
			if originalWd != "" {
				_ = os.Chdir(originalWd)
			}
		}()
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
