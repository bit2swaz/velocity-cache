package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// NewLinkCommand exposes the link command for registration.
func NewLinkCommand() *cobra.Command {
	return newLinkCommand()
}

func newLinkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link a project ID to the current velocity configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Please paste your Project ID from your dashboard: ")

			reader := bufio.NewReader(os.Stdin)
			projectIDRaw, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read project id: %w", err)
			}

			projectID := strings.TrimSpace(projectIDRaw)
			if projectID == "" {
				return fmt.Errorf("project id cannot be empty")
			}

			configFilePath := "velocity.config.json"
			configMap := make(map[string]interface{})

			data, err := os.ReadFile(configFilePath)
			if err == nil {
				if err := json.Unmarshal(data, &configMap); err != nil {
					return fmt.Errorf("Failed to parse config: %w", err)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("Failed to read config: %w", err)
			}

			configMap["project_id"] = projectID

			updated, err := json.MarshalIndent(configMap, "", "  ")
			if err != nil {
				return fmt.Errorf("Failed to encode config: %w", err)
			}
			updated = append(updated, '\n')

			if err := os.WriteFile(configFilePath, updated, 0o644); err != nil {
				return fmt.Errorf("Failed to write config: %w", err)
			}

			fmt.Println("[VelocityCache] Project ID linked successfully.")
			return nil
		},
	}

	return cmd
}
