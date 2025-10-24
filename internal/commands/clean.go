package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bit2swaz/velocity-cache/internal/engine"
)

const cachePath = ".velocity/cache"

func newCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove the local velocity cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := engine.CleanLocal(); err != nil {
				return fmt.Errorf("remove %s: %w", cachePath, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", prefix(), infoStyle.Sprintf("Removed %s", cachePath))
			return nil
		},
	}
	return cmd
}
