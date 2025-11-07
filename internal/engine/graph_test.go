package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

func TestBuildTaskGraphCycleDetection(t *testing.T) {
	cfg := &config.Config{
		Tasks: map[string]config.TaskConfig{
			"build": {
				DependsOn: []string{"test"},
			},
			"test": {
				DependsOn: []string{"build"},
			},
		},
	}

	pkg := &Package{
		Name:            "pkg",
		Path:            "packages/app",
		PackageJsonPath: "packages/app/package.json",
	}

	packages := map[string]*Package{pkg.Name: pkg}

	_, err := BuildTaskGraph("build", pkg, packages, cfg, map[string]bool{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle", "should mention cycle in error")
}

func TestBuildTaskGraphTopologicalDependencies(t *testing.T) {
	cfg := &config.Config{
		Tasks: map[string]config.TaskConfig{
			"build": {
				DependsOn: []string{"^build"},
			},
		},
	}

	libPkg := &Package{
		Name:            "@repo/lib",
		Path:            "packages/lib",
		PackageJsonPath: "packages/lib/package.json",
	}

	appPkg := &Package{
		Name:             "@repo/app",
		Path:             "packages/app",
		PackageJsonPath:  "packages/app/package.json",
		InternalDepNames: []string{libPkg.Name},
		InternalDeps:     []*Package{libPkg},
	}

	packages := map[string]*Package{
		libPkg.Name: libPkg,
		appPkg.Name: appPkg,
	}

	node, err := BuildTaskGraph("build", appPkg, packages, cfg, map[string]bool{})
	require.NoError(t, err)

	assert.Equal(t, "packages/app#build", node.ID)
	require.Len(t, node.Dependencies, 1)
	assert.Equal(t, "packages/lib#build", node.Dependencies[0].ID)
	assert.Equal(t, libPkg, node.Dependencies[0].Package)
	assert.Equal(t, "build", node.Dependencies[0].TaskName)
}
