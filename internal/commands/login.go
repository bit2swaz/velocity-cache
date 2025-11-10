package commands

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bit2swaz/velocity-cache/internal/auth"
)

// NewLoginCommand returns the login command so other packages can register it if needed.
func NewLoginCommand() *cobra.Command {
	return newLoginCommand()
}

func newLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save your API token for future commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Please paste your API token from your dashboard: ")

			byteToken, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("read token: %w", err)
			}

			token := string(byteToken)

			if err := auth.SaveToken(token); err != nil {
				return err
			}

			fmt.Println("[VelocityCache] Token saved successfully.")
			return nil
		},
	}

	return cmd
}
