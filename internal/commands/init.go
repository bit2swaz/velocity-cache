package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const configFileName = "velocity.config.json"
const exampleTemplateName = "velocity.config.json.example"

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize velocity in the current directory",
		Long:  "Generates a sample velocity.config.json.example for quick setup.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd)
		},
	}
	return cmd
}

func runInit(cmd *cobra.Command) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}

	targetPath := filepath.Join(wd, configFileName)

	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("%s already exists", configFileName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check %s: %w", configFileName, err)
	}

	templatePath, err := locateTemplate()
	if err != nil {
		return err
	}
	templateContents, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read %s template: %w", exampleTemplateName, err)
	}

	if err := os.WriteFile(targetPath, templateContents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configFileName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", prefix(), infoStyle.Sprintf("Generated %s", configFileName))
	return nil
}

func locateTemplate() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, exampleTemplateName), nil
}
