package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bit2swaz/velocity-cache/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvHashing(t *testing.T) {
	t.Setenv("NODE_ENV", "")
	cfg := config.TaskConfig{
		Command: "npm run build",
		EnvKeys: []string{"NODE_ENV"},
	}

	hash1, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "first hash should succeed")
	hash2, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "second hash should succeed")
	assert.Equal(t, hash1, hash2, "expected deterministic hash")

	t.Setenv("NODE_ENV", "production")
	hash3, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "hash with env change should succeed")
	assert.NotEqual(t, hash1, hash3, "expected env change to alter hash")
}

func TestCommandHashing(t *testing.T) {
	cfg := config.TaskConfig{
		Command: "npm run build",
	}

	hash1, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "first hash should succeed")
	hash2, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "second hash should succeed")
	assert.Equal(t, hash1, hash2, "expected deterministic hash")

	cfg.Command = "npm run test"
	hash3, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "hash after command change should succeed")
	assert.NotEqual(t, hash1, hash3, "expected command change to alter hash")
}

func TestFileInputsAffectHash(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0o644))

	cfg := config.TaskConfig{
		Command: "npm run build",
		Inputs:  []string{"*.txt"},
	}

	hash1, err := GenerateCacheKey(cfg, nil, tmpDir)
	require.NoError(t, err, "hash with initial files should succeed")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("c"), 0o644))

	hash2, err := GenerateCacheKey(cfg, nil, tmpDir)
	require.NoError(t, err, "hash after adding file should succeed")
	assert.NotEqual(t, hash1, hash2, "adding matching file should alter hash")
}

func TestGitignoreFiltersFiles(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("ignored.txt\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "included.txt"), []byte("include"), 0o644))

	cfg := config.TaskConfig{
		Command: "npm run build",
		Inputs:  []string{"*.txt"},
	}

	hash1, err := GenerateCacheKey(cfg, nil, tmpDir)
	require.NoError(t, err, "hash with ignored file absent should succeed")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ignored.txt"), []byte("ignore"), 0o644))

	hash2, err := GenerateCacheKey(cfg, nil, tmpDir)
	require.NoError(t, err, "hash with ignored file should succeed")
	assert.Equal(t, hash1, hash2, "ignored file should not alter hash")
}

func TestGenerateCacheKeyIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	write := func(rel, contents string) {
		path := filepath.Join(tmpDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	write(".gitignore", "ignored.txt\nignored-dir/\n")
	write("src/a.txt", "alpha")
	write("src/b.txt", "beta")
	write("ignored.txt", "ignored contents")
	write("ignored-dir/c.txt", "ignored directory file")

	cfg := config.TaskConfig{
		Command: "npm run build",
		Inputs:  []string{"**/*.txt"},
	}

	run := func() string {
		t.Helper()
		hash, err := GenerateCacheKey(cfg, nil, tmpDir)
		require.NoError(t, err, "GenerateCacheKey should succeed")
		return hash
	}

	hash1 := run()

	hash2 := run()
	assert.Equal(t, hash1, hash2, "determinism check should pass")

	write("src/a.txt", "alpha-modified")
	hash3 := run()
	assert.NotEqual(t, hash1, hash3, "changing tracked file should alter hash")

	write("src/a.txt", "alpha")
	hash4Before := run()
	assert.Equal(t, hash1, hash4Before, "restoring tracked file should restore hash")

	write("ignored.txt", "updated ignored contents")
	hash4 := run()
	assert.Equal(t, hash1, hash4, "ignored file change should not alter hash")

	write("src/new.txt", "new tracked file")
	hash5 := run()
	assert.NotEqual(t, hash1, hash5, "adding tracked file should alter hash")
}

func TestGenerateTaskNodeCacheKeyIncludesNodeIdentity(t *testing.T) {
	cfg := config.TaskConfig{Command: "npm run build"}

	nodeA := &TaskNode{
		ID:         "packages/app#build",
		TaskName:   "build",
		TaskConfig: cfg,
		Package: &Package{
			Name: "@repo/app",
			Path: "packages/app",
		},
	}

	nodeB := &TaskNode{
		ID:         "packages/lib#build",
		TaskName:   "build",
		TaskConfig: cfg,
		Package: &Package{
			Name: "@repo/lib",
			Path: "packages/lib",
		},
	}

	keyA, err := GenerateTaskNodeCacheKey(nodeA, nil)
	require.NoError(t, err, "GenerateTaskNodeCacheKey should succeed for nodeA")
	keyB, err := GenerateTaskNodeCacheKey(nodeB, nil)
	require.NoError(t, err, "GenerateTaskNodeCacheKey should succeed for nodeB")
	assert.NotEqual(t, keyA, keyB, "distinct packages should yield unique cache keys")
}

func TestGenerateCacheKeyDependsOnDependencyKeys(t *testing.T) {
	cfg := config.TaskConfig{Command: "npm run build"}

	base, err := GenerateCacheKey(cfg, nil, "")
	require.NoError(t, err, "base key should compute")

	withDepsOrder1, err := GenerateCacheKey(cfg, []string{"dep-b", "dep-a"}, "")
	require.NoError(t, err, "key with deps should compute")
	withDepsOrder2, err := GenerateCacheKey(cfg, []string{"dep-a", "dep-b"}, "")
	require.NoError(t, err, "key with deps in different order should compute")

	assert.NotEqual(t, base, withDepsOrder1, "dependency keys should influence hash")
	assert.Equal(t, withDepsOrder1, withDepsOrder2, "dependency order should not matter")
}

func TestGenerateTaskNodeCacheKeyIncludesDependencyKeys(t *testing.T) {
	cfg := config.TaskConfig{Command: "npm run build"}

	depNode := &TaskNode{
		ID:         "packages/lib#build",
		TaskName:   "build",
		TaskConfig: cfg,
		Package: &Package{
			Name: "@repo/lib",
			Path: "packages/lib",
		},
	}

	rootNode := &TaskNode{
		ID:         "packages/app#build",
		TaskName:   "build",
		TaskConfig: cfg,
		Package: &Package{
			Name: "@repo/app",
			Path: "packages/app",
		},
		Dependencies: []*TaskNode{depNode},
	}

	depKey, err := GenerateTaskNodeCacheKey(depNode, nil)
	require.NoError(t, err, "dependency key should compute")

	rootWithoutDeps, err := GenerateTaskNodeCacheKey(rootNode, nil)
	require.NoError(t, err, "root key without deps should compute")

	rootWithDeps, err := GenerateTaskNodeCacheKey(rootNode, []string{depKey})
	require.NoError(t, err, "root key with deps should compute")

	assert.NotEqual(t, rootWithoutDeps, rootWithDeps, "including dependency keys should alter hash")
}

func TestDependencyHashPropagation(t *testing.T) {
	tmpDir := t.TempDir()

	write := func(rel, contents string) {
		t.Helper()
		path := filepath.Join(tmpDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	write("packages/a/input.txt", "initial-a")
	write("packages/b/input.txt", "initial-b")

	packageAPath := filepath.Join(tmpDir, "packages", "a")
	packageBPath := filepath.Join(tmpDir, "packages", "b")

	wd, err := os.Getwd()
	require.NoError(t, err, "get working directory")
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd), "restore working directory")
	})

	require.NoError(t, os.Chdir(tmpDir), "change to temp directory")

	taskA := &TaskNode{
		ID:         "packages/a#build",
		TaskName:   "build",
		TaskConfig: config.TaskConfig{Command: "echo build-a", Inputs: []string{"*.txt"}},
		Package: &Package{
			Name: "pkg-a",
			Path: packageAPath,
		},
	}

	taskB := &TaskNode{
		ID:         "packages/b#build",
		TaskName:   "build",
		TaskConfig: config.TaskConfig{Command: "echo build-b", Inputs: []string{"*.txt"}},
		Package: &Package{
			Name: "pkg-b",
			Path: packageBPath,
		},
		Dependencies: []*TaskNode{taskA},
	}

	executeTask := func(node *TaskNode) string {
		cache := make(map[string]string)
		var run func(*TaskNode) string
		run = func(n *TaskNode) string {
			if key, ok := cache[n.ID]; ok {
				return key
			}
			depKeys := make([]string, 0, len(n.Dependencies))
			for _, dep := range n.Dependencies {
				depKeys = append(depKeys, run(dep))
			}
			key, err := GenerateTaskNodeCacheKey(n, depKeys)
			require.NoError(t, err, "generate cache key for %s", n.ID)
			cache[n.ID] = key
			return key
		}
		return run(node)
	}

	hashA1 := executeTask(taskA)
	hashB1 := executeTask(taskB)

	write("packages/a/input.txt", "updated-a")

	hashA2 := executeTask(taskA)
	hashB2 := executeTask(taskB)

	assert.NotEqual(t, hashA1, hashA2, "task A hash should change when its inputs change")
	assert.NotEqual(t, hashB1, hashB2, "task B hash should change when dependency hash changes")
}
