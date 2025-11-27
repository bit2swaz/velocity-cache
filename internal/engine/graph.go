package engine

import (
	"fmt"
	"strings"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

type TaskNode struct {
	ID           string
	Package      *Package
	TaskName     string
	TaskConfig   config.TaskConfig
	Dependencies []*TaskNode

	State     int
	CacheKey  string
	LastError error
}

func BuildTaskGraph(targetTaskName string, targetPackage *Package, allPackages map[string]*Package, cfg *config.Config, visiting map[string]bool) (*TaskNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("task graph requires configuration")
	}
	if targetPackage == nil {
		return nil, fmt.Errorf("task graph requires a target package")
	}

	if allPackages != nil {
		if _, ok := allPackages[targetPackage.Name]; !ok {

			return nil, fmt.Errorf("package %q not found in discovered packages", targetPackage.Name)
		}
	}

	if visiting == nil {
		visiting = make(map[string]bool)
	}

	nodeID := fmt.Sprintf("%s#%s", targetPackage.Path, targetTaskName)
	if visiting[nodeID] {
		return nil, fmt.Errorf("detected cycle while building task graph at %s", nodeID)
	}

	taskCfg, ok := cfg.Pipeline[targetTaskName]
	if !ok {
		return nil, fmt.Errorf("task %q not defined in configuration", targetTaskName)
	}

	visiting[nodeID] = true
	defer delete(visiting, nodeID)

	if len(targetPackage.InternalDeps) == 0 && len(targetPackage.InternalDepNames) > 0 {
		if allPackages == nil {
			return nil, fmt.Errorf("package %q missing resolved dependencies", targetPackage.Name)
		}

		deps := make([]*Package, 0, len(targetPackage.InternalDepNames))
		for _, name := range targetPackage.InternalDepNames {
			depPkg, exists := allPackages[name]
			if !exists {
				return nil, fmt.Errorf("package %q depends on unknown package %q", targetPackage.Name, name)
			}
			deps = append(deps, depPkg)
		}
		targetPackage.InternalDeps = deps
	}

	node := &TaskNode{
		ID:         nodeID,
		Package:    targetPackage,
		TaskName:   targetTaskName,
		TaskConfig: taskCfg,
	}

	depSeen := make(map[string]struct{})

	for _, depRef := range taskCfg.DependsOn {
		depRef = strings.TrimSpace(depRef)
		if depRef == "" {
			continue
		}

		if strings.HasPrefix(depRef, "^") {
			depTaskName := strings.TrimPrefix(depRef, "^")
			if depTaskName == "" {
				return nil, fmt.Errorf("task %q dependency %q missing task name", targetTaskName, depRef)
			}

			for _, depPkg := range targetPackage.InternalDeps {
				child, err := BuildTaskGraph(depTaskName, depPkg, allPackages, cfg, visiting)
				if err != nil {
					return nil, err
				}
				if _, exists := depSeen[child.ID]; exists {
					continue
				}
				depSeen[child.ID] = struct{}{}
				node.Dependencies = append(node.Dependencies, child)
			}
			continue
		}

		child, err := BuildTaskGraph(depRef, targetPackage, allPackages, cfg, visiting)
		if err != nil {
			return nil, err
		}
		if _, exists := depSeen[child.ID]; exists {
			continue
		}
		depSeen[child.ID] = struct{}{}
		node.Dependencies = append(node.Dependencies, child)
	}

	return node, nil
}
