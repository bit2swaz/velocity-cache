package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Package struct {
	Name             string
	Path             string
	PackageJsonPath  string
	InternalDepNames []string
	InternalDeps     []*Package
}

func DiscoverPackages(patterns []string) (map[string]*Package, error) {
	discovered := make(map[string]*Package)

	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}

		globPattern := trimmed
		if !strings.HasSuffix(trimmed, "package.json") {
			globPattern = filepath.Join(trimmed, "package.json")
		}

		matches, err := doublestar.FilepathGlob(globPattern)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", globPattern, err)
		}

		for _, pkgJSONPath := range matches {
			pkg, err := readPackageJson(pkgJSONPath)
			if err != nil {
				return nil, fmt.Errorf("parse package.json %q: %w", pkgJSONPath, err)
			}

			if existing, exists := discovered[pkg.Name]; exists {

				if existing.PackageJsonPath != filepath.Clean(pkgJSONPath) {
					return nil, fmt.Errorf("duplicate package %q found at %q and %q", pkg.Name, existing.PackageJsonPath, pkg.PackageJsonPath)
				}
				continue
			}

			discovered[pkg.Name] = pkg
		}
	}

	return discovered, nil
}

type packageJson struct {
	Name                 string            `json:"name"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

func readPackageJson(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var parsed packageJson
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if parsed.Name == "" {
		return nil, fmt.Errorf("missing name field")
	}

	deps := collectWorkspaceDeps(parsed.Dependencies, parsed.DevDependencies, parsed.OptionalDependencies, parsed.PeerDependencies)

	pkg := &Package{
		Name:             parsed.Name,
		Path:             filepath.Dir(path),
		PackageJsonPath:  filepath.Clean(path),
		InternalDepNames: deps,
	}

	return pkg, nil
}

func collectWorkspaceDeps(depGroups ...map[string]string) []string {
	depSet := make(map[string]struct{})

	for _, group := range depGroups {
		for name, version := range group {
			if strings.HasPrefix(version, "workspace:") {
				depSet[name] = struct{}{}
			}
		}
	}

	if len(depSet) == 0 {
		return nil
	}

	deps := make([]string, 0, len(depSet))
	for name := range depSet {
		deps = append(deps, name)
	}

	slices.Sort(deps)
	return deps
}

func BuildPackageGraph(packages map[string]*Package) error {
	for _, pkg := range packages {
		if len(pkg.InternalDepNames) == 0 {
			pkg.InternalDeps = nil
			continue
		}

		deps := make([]*Package, 0, len(pkg.InternalDepNames))
		for _, depName := range pkg.InternalDepNames {
			depPkg, ok := packages[depName]
			if !ok {
				return fmt.Errorf("package %q depends on unknown package %q", pkg.Name, depName)
			}
			deps = append(deps, depPkg)
		}

		pkg.InternalDeps = deps
	}

	return nil
}
