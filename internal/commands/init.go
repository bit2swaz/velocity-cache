package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	vc "github.com/bit2swaz/velocity-cache"
)

const configFileName = "velocity.config.json"

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

	contents := vc.VelocityConfigTemplate()

	if err := os.WriteFile(targetPath, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configFileName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", prefix(), infoStyle.Sprintf("Generated %s", configFileName))
	return nil
}
