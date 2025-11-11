package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

			configBytes, err := ioutil.ReadFile(configFileName)
			if err != nil {
				return fmt.Errorf("read %s: %w", configFileName, err)
			}

			var configMap map[string]interface{}
			if err := json.Unmarshal(configBytes, &configMap); err != nil {
				return fmt.Errorf("parse %s: %w", configFileName, err)
			}

			configMap["project_id"] = projectID

			updated, err := json.MarshalIndent(configMap, "", "  ")
			if err != nil {
				return fmt.Errorf("encode %s: %w", configFileName, err)
			}
			updated = append(updated, '\n')

			if err := ioutil.WriteFile(configFileName, updated, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", configFileName, err)
			}

			fmt.Println("[VelocityCache] Project ID linked successfully.")
			return nil
		},
	}

	return cmd
}
