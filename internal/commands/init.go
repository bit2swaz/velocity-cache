package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

const configFileName = "velocity.yml"

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a velocity.yml configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd)
		},
	}
}

func runInit(cmd *cobra.Command) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}

	targetPath := filepath.Join(wd, configFileName)

	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("%s already exists", configFileName)
	}

	// 1. Try Turbo Import
	turboPath := filepath.Join(wd, "turbo.json")
	packageJSONPath := filepath.Join(wd, "package.json")
	if info, err := os.Stat(turboPath); err == nil && !info.IsDir() {
		cfg, err := parseTurboConfig(turboPath, packageJSONPath)
		if err != nil {
			return fmt.Errorf("parse turbo.json: %w", err)
		}
		return writeYaml(targetPath, cfg)
	}

	// 2. Try Language Detectors
	if cfg, ok := detectLanguageProject(wd); ok {
		return writeYaml(targetPath, cfg)
	}

	// 3. Default Template
	defaultCfg := &config.Config{
		Version:   1,
		ProjectID: "my-project",
		Remote: config.RemoteConfig{
			Enabled: true,
			URL:     "${VC_SERVER_URL}",
			Token:   "${VC_AUTH_TOKEN}",
		},
		Pipeline: map[string]config.TaskConfig{
			"build": {
				Command:   "npm run build",
				Inputs:    []string{"src/**", "package.json"},
				Outputs:   []string{"dist/**"},
				DependsOn: []string{"^build"},
			},
		},
	}
	return writeYaml(targetPath, defaultCfg)
}

func writeYaml(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", path)
	return nil
}

// --- Language Detection (Simplified for V3) ---

func detectLanguageProject(root string) (*config.Config, bool) {
	// Python
	if _, err := os.Stat(filepath.Join(root, "requirements.txt")); err == nil {
		return &config.Config{
			Version: 1,
			Pipeline: map[string]config.TaskConfig{
				"test": {Command: "pytest", Inputs: []string{"**/*.py"}},
			},
		}, true
	}
	// Go
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return &config.Config{
			Version: 1,
			Pipeline: map[string]config.TaskConfig{
				"build": {Command: "go build ./...", Inputs: []string{"**/*.go"}},
			},
		}, true
	}
	return nil, false
}

// --- Turbo Parser (Adapted for YAML Config) ---

type turboFile struct {
	Pipeline map[string]struct {
		DependsOn []string `json:"dependsOn"`
		Inputs    []string `json:"inputs"`
		Outputs   []string `json:"outputs"`
	} `json:"pipeline"`
}

func parseTurboConfig(turboPath, packageJSONPath string) (*config.Config, error) {
	data, _ := os.ReadFile(turboPath)
	var t turboFile
	json.Unmarshal(data, &t)

	pipeline := make(map[string]config.TaskConfig)
	for name, task := range t.Pipeline {
		pipeline[name] = config.TaskConfig{
			Command:   "npm run " + name,
			DependsOn: task.DependsOn,
			Inputs:    task.Inputs,
			Outputs:   task.Outputs,
		}
	}

	return &config.Config{
		Version:  1,
		Remote:   config.RemoteConfig{Enabled: true, URL: "${VC_SERVER_URL}", Token: "${VC_AUTH_TOKEN}"},
		Pipeline: pipeline,
	}, nil
}
