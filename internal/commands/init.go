package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	turboPath := filepath.Join(wd, "turbo.json")
	packageJSONPath := filepath.Join(wd, "package.json")
	if info, err := os.Stat(turboPath); err == nil && !info.IsDir() {
		cfg, err := parseTurboConfig(turboPath, packageJSONPath)
		if err != nil {
			return fmt.Errorf("parse turbo.json: %w", err)
		}
		return writeYaml(cmd, targetPath, cfg)
	}

	if cfg, ok := detectLanguageProject(wd); ok {
		return writeYaml(cmd, targetPath, cfg)
	}

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
	return writeYaml(cmd, targetPath, defaultCfg)
}

func writeYaml(cmd *cobra.Command, path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	cmd.Printf("Generated %s\n", filepath.Base(path))
	return nil
}

func detectLanguageProject(root string) (*config.Config, bool) {
	if _, err := os.Stat(filepath.Join(root, "requirements.txt")); err == nil {
		return &config.Config{
			Version: 1,
			Pipeline: map[string]config.TaskConfig{
				"test": {
					Command: "pytest",
					Inputs:  []string{"**/*.py", "requirements.txt", "poetry.lock"},
					Outputs: []string{".venv/", ".cache/", "__pycache__/"},
				},
				"lint": {
					Command: "flake8",
				},
			},
		}, true
	}

	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err == nil {
		return &config.Config{
			Version: 1,
			Pipeline: map[string]config.TaskConfig{
				"build": {
					Command: "cargo build --release",
					Inputs:  []string{"src/**/*.rs", "Cargo.toml", "Cargo.lock"},
					Outputs: []string{"target/"},
				},
				"test": {
					Command: "cargo test",
				},
			},
		}, true
	}

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		moduleName := "app"
		if f, err := os.Open(filepath.Join(root, "go.mod")); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "module ") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						modulePath := parts[1]
						moduleParts := strings.Split(modulePath, "/")
						moduleName = moduleParts[len(moduleParts)-1]
					}
					break
				}
			}
			f.Close()
		}

		return &config.Config{
			Version: 1,
			Pipeline: map[string]config.TaskConfig{
				"build": {
					Command: fmt.Sprintf("go build -o bin/%s ./cmd/...", moduleName),
					Inputs:  []string{"**/*.go", "go.mod", "go.sum"},
					Outputs: []string{"bin/"},
				},
			},
		}, true
	}
	return nil, false
}

type turboFile struct {
	Pipeline map[string]struct {
		DependsOn []string `json:"dependsOn"`
		Inputs    []string `json:"inputs"`
		Outputs   []string `json:"outputs"`
		Env       []string `json:"env"`
	} `json:"pipeline"`
}

type packageJSON struct {
	Workspaces []string `json:"workspaces"`
}

func parseTurboConfig(turboPath, packageJSONPath string) (*config.Config, error) {
	data, _ := os.ReadFile(turboPath)
	var t turboFile
	json.Unmarshal(data, &t)

	var workspaces []string
	if pkgData, err := os.ReadFile(packageJSONPath); err == nil {
		var p packageJSON
		if err := json.Unmarshal(pkgData, &p); err == nil {
			workspaces = p.Workspaces
		}
	}

	pipeline := make(map[string]config.TaskConfig)
	for name, task := range t.Pipeline {
		pipeline[name] = config.TaskConfig{
			Command:   "npm run " + name,
			DependsOn: task.DependsOn,
			Inputs:    task.Inputs,
			Outputs:   task.Outputs,
			EnvKeys:   task.Env,
		}
	}

	return &config.Config{
		Version:  1,
		Remote:   config.RemoteConfig{Enabled: true, URL: "${VC_SERVER_URL}", Token: "${VC_AUTH_TOKEN}"},
		Pipeline: pipeline,
		Packages: workspaces,
	}, nil
}
