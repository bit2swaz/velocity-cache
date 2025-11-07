package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	vc "github.com/bit2swaz/velocity-cache"
	"github.com/bit2swaz/velocity-cache/internal/config"
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

	turboPath := filepath.Join(wd, "turbo.json")
	packageJSONPath := filepath.Join(wd, "package.json")

	if info, err := os.Stat(turboPath); err == nil && !info.IsDir() {
		cfg, err := parseTurboConfig(turboPath, packageJSONPath)
		if err != nil {
			return fmt.Errorf("generate velocity config from turbo.json: %w", err)
		}

		if err := writeConfig(targetPath, cfg); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "[VelocityCache] Detected turbo.json and created a velocity.config.json for you. Please review it for accuracy.")
		return nil
	}

	if cfg, ok := detectLanguageProject(wd); ok {
		if err := writeConfig(targetPath, cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", prefix(), infoStyle.Sprintf("Generated %s", configFileName))
		return nil
	}

	contents := vc.VelocityConfigTemplate()

	if err := os.WriteFile(targetPath, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configFileName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", prefix(), infoStyle.Sprintf("Generated %s", configFileName))
	return nil
}

func writeConfig(path string, cfg *config.Config) error {
	contents, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal velocity config: %w", err)
	}
	contents = append(contents, '\n')

	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configFileName, err)
	}

	return nil
}

type languageDetector func(root string) (*config.Config, bool, error)

func detectLanguageProject(root string) (*config.Config, bool) {
	detectors := []languageDetector{
		detectPythonProject,
		detectRustProject,
		detectGoProject,
	}

	for _, detector := range detectors {
		cfg, ok, err := detector(root)
		if err != nil {
			return nil, false
		}
		if ok {
			return cfg, true
		}
	}

	return nil, false
}

func detectPythonProject(root string) (*config.Config, bool, error) {
	requirements := filepath.Join(root, "requirements.txt")
	poetry := filepath.Join(root, "poetry.lock")

	if _, err := os.Stat(requirements); err == nil {
		return pythonConfig(), true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}

	if _, err := os.Stat(poetry); err == nil {
		return pythonConfig(), true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}

	return nil, false, nil
}

func pythonConfig() *config.Config {
	return &config.Config{
		Tasks: map[string]config.TaskConfig{
			"test": {
				Command:   "pytest",
				Inputs:    []string{"**/*.py", "requirements.txt", "poetry.lock"},
				Outputs:   []string{".venv/", ".cache/", "__pycache__/"},
				DependsOn: nil,
				EnvKeys:   nil,
			},
			"lint": {
				Command: "flake8",
				Inputs:  []string{"**/*.py", "requirements.txt", "poetry.lock"},
				Outputs: []string{".venv/", ".cache/", "__pycache__/"},
			},
		},
	}
}

func detectRustProject(root string) (*config.Config, bool, error) {
	cargo := filepath.Join(root, "Cargo.toml")
	if _, err := os.Stat(cargo); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	cfg := &config.Config{
		Tasks: map[string]config.TaskConfig{
			"build": {
				Command: "cargo build --release",
				Inputs:  []string{"src/**/*.rs", "Cargo.toml", "Cargo.lock"},
				Outputs: []string{"target/"},
			},
			"test": {
				Command: "cargo test",
				Inputs:  []string{"src/**/*.rs", "Cargo.toml", "Cargo.lock"},
				Outputs: []string{"target/"},
			},
		},
	}
	return cfg, true, nil
}

func detectGoProject(root string) (*config.Config, bool, error) {
	gomod := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(gomod)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	binaryName := deriveGoBinaryName(root, string(data))

	cfg := &config.Config{
		Tasks: map[string]config.TaskConfig{
			"build": {
				Command: fmt.Sprintf("go build -o bin/%s ./cmd/...", binaryName),
				Inputs:  []string{"**/*.go", "go.mod", "go.sum"},
				Outputs: []string{"bin/"},
			},
		},
	}
	return cfg, true, nil
}

func deriveGoBinaryName(root, goModContents string) string {
	moduleName := ""
	for _, line := range strings.Split(goModContents, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			moduleName = strings.Trim(moduleName, "\"'")
			break
		}
	}

	if moduleName != "" {
		segments := strings.Split(moduleName, "/")
		if last := strings.TrimSpace(segments[len(segments)-1]); last != "" {
			return last
		}
	}

	binaryName := strings.TrimSpace(filepath.Base(root))
	if binaryName == "" || binaryName == string(filepath.Separator) {
		return "app"
	}
	return binaryName
}

type turboPipelineTask struct {
	DependsOn []string `json:"dependsOn"`
	Inputs    []string `json:"inputs"`
	Outputs   []string `json:"outputs"`
	Env       []string `json:"env"`
}

type turboFile struct {
	Pipeline map[string]turboPipelineTask `json:"pipeline"`
}

func parseTurboConfig(turboPath, packageJSONPath string) (*config.Config, error) {
	turboBytes, err := os.ReadFile(turboPath)
	if err != nil {
		return nil, fmt.Errorf("read turbo.json: %w", err)
	}

	var turboCfg turboFile
	if err := json.Unmarshal(turboBytes, &turboCfg); err != nil {
		return nil, fmt.Errorf("unmarshal turbo.json: %w", err)
	}

	if len(turboCfg.Pipeline) == 0 {
		return nil, fmt.Errorf("turbo.json pipeline is empty")
	}

	packages, err := extractWorkspaces(packageJSONPath)
	if err != nil {
		return nil, err
	}

	tasks := make(map[string]config.TaskConfig, len(turboCfg.Pipeline))
	for name, task := range turboCfg.Pipeline {
		tasks[name] = config.TaskConfig{
			Command:   fmt.Sprintf("npm run %s", name),
			DependsOn: cloneStrings(task.DependsOn),
			Inputs:    cloneStrings(task.Inputs),
			Outputs:   cloneStrings(task.Outputs),
			EnvKeys:   cloneStrings(task.Env),
		}
	}

	return &config.Config{
		Packages: packages,
		Tasks:    tasks,
	}, nil
}

func extractWorkspaces(packageJSONPath string) ([]string, error) {
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("package.json not found in project root")
		}
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	var pkg struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}

	if len(pkg.Workspaces) == 0 {
		return nil, fmt.Errorf("package.json missing workspaces field")
	}

	var arr []string
	if err := json.Unmarshal(pkg.Workspaces, &arr); err == nil {
		return normalizeStrings(arr), nil
	}

	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(pkg.Workspaces, &obj); err == nil && len(obj.Packages) > 0 {
		return normalizeStrings(obj.Packages), nil
	}

	var generic map[string][]string
	if err := json.Unmarshal(pkg.Workspaces, &generic); err == nil {
		var collected []string
		for _, entries := range generic {
			collected = append(collected, entries...)
		}
		if len(collected) > 0 {
			return normalizeStrings(collected), nil
		}
	}

	return nil, fmt.Errorf("could not parse workspaces from package.json")
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func normalizeStrings(values []string) []string {
	uniq := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		uniq[v] = struct{}{}
	}

	result := make([]string, 0, len(uniq))
	for v := range uniq {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
