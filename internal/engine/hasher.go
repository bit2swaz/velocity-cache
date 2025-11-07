package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	ignore "github.com/sabhiram/go-gitignore"

	"github.com/bit2swaz/velocity-cache/internal/config"
)

// GenerateCacheKey returns a deterministic cache key for the supplied script config.
func GenerateCacheKey(cfg config.TaskConfig, depCacheKeys []string, packagePath string) (string, error) {
	localHash, err := computeLocalHash(cfg, packagePath)
	if err != nil {
		return "", err
	}

	parts := []string{localHash}
	if len(depCacheKeys) > 0 {
		sorted := make([]string, len(depCacheKeys))
		copy(sorted, depCacheKeys)
		sort.Strings(sorted)
		depString := strings.Join(sorted, "|")
		parts = append(parts, depString)
	}

	return hashString(strings.Join(parts, "|")), nil
}

func computeLocalHash(cfg config.TaskConfig, packagePath string) (string, error) {
	var envHash string
	if len(cfg.EnvKeys) > 0 {
		envPairs := make([]string, 0, len(cfg.EnvKeys))
		for _, key := range cfg.EnvKeys {
			envPairs = append(envPairs, key+"="+os.Getenv(key))
		}
		sort.Strings(envPairs)
		envHash = hashString(strings.Join(envPairs, "|"))
	}

	commandHash := hashString(cfg.Command)

	files, err := collectInputFiles(cfg.Inputs, packagePath)
	if err != nil {
		return "", err
	}

	fileHashes, err := hashFiles(files)
	if err != nil {
		return "", err
	}

	var filesHash string
	if len(files) > 0 {
		entries := make([]string, 0, len(files))
		for _, path := range files {
			sum, ok := fileHashes[path]
			if !ok {
				continue
			}
			entries = append(entries, path+":"+sum)
		}
		filesHash = hashString(strings.Join(entries, "|"))
	}

	parts := make([]string, 0, 3)
	if envHash != "" {
		parts = append(parts, "env:"+envHash)
	}
	parts = append(parts, "cmd:"+commandHash)
	if filesHash != "" {
		parts = append(parts, "files:"+filesHash)
	}

	return strings.Join(parts, "|"), nil
}

func collectInputFiles(patterns []string, packagePath string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	originalWd := ""
	if packagePath != "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd: %w", err)
		}
		if err := os.Chdir(packagePath); err != nil {
			return nil, fmt.Errorf("chdir to %s: %w", packagePath, err)
		}
		originalWd = wd
		defer func() {
			if originalWd != "" {
				_ = os.Chdir(originalWd)
			}
		}()
	}

	matcher, err := loadGitignore()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	files := make([]string, 0)

	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}

		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}

		for _, match := range matches {
			relativePath := filepath.Clean(match)

			info, err := os.Stat(relativePath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("stat %q: %w", relativePath, err)
			}

			if info.IsDir() {
				continue
			}

			if matcher != nil && matcher.MatchesPath(relativePath) {
				continue
			}

			resolvedPath := relativePath
			if packagePath != "" && !filepath.IsAbs(resolvedPath) {
				resolvedPath = filepath.Clean(filepath.Join(packagePath, resolvedPath))
			}

			if _, ok := seen[resolvedPath]; ok {
				continue
			}

			seen[resolvedPath] = struct{}{}
			files = append(files, resolvedPath)
		}
	}

	sort.Strings(files)

	return files, nil
}

func loadGitignore() (*ignore.GitIgnore, error) {
	_, err := os.Stat(".gitignore")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat .gitignore: %w", err)
	}

	matcher, err := ignore.CompileIgnoreFile(".gitignore")
	if err != nil {
		return nil, fmt.Errorf("compile .gitignore: %w", err)
	}

	return matcher, nil
}

type fileHashResult struct {
	path string
	sum  string
	err  error
}

func hashFiles(paths []string) (map[string]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	workerCount := runtime.NumCPU()
	if workerCount < 1 {
		workerCount = 1
	}

	jobs := make(chan string)
	results := make(chan fileHashResult, len(paths))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				sum, err := hashFile(path)
				results <- fileHashResult{path: path, sum: sum, err: err}
			}
		}()
	}

	go func() {
		for _, path := range paths {
			jobs <- path
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	hashes := make(map[string]string, len(paths))
	var hashErr error
	for res := range results {
		if res.err != nil {
			if hashErr == nil {
				hashErr = res.err
			}
			continue
		}
		hashes[res.path] = res.sum
	}

	if hashErr != nil {
		return nil, hashErr
	}

	return hashes, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// GenerateTaskNodeCacheKey produces a cache key that is unique to a task node,
// incorporating both the task configuration hash and the task's identity
// (package path + task name). This prevents collisions when multiple packages
// share identical task definitions.
func GenerateTaskNodeCacheKey(node *TaskNode, depCacheKeys []string) (string, error) {
	if node == nil {
		return "", fmt.Errorf("task node is nil")
	}

	packagePath := ""
	if node.Package != nil {
		packagePath = node.Package.Path
	}

	baseKey, err := GenerateCacheKey(node.TaskConfig, depCacheKeys, packagePath)
	if err != nil {
		return "", err
	}

	identifier := node.ID
	if strings.TrimSpace(identifier) == "" {
		if node.Package != nil && node.Package.Path != "" && node.TaskName != "" {
			identifier = fmt.Sprintf("%s#%s", node.Package.Path, node.TaskName)
		} else if node.TaskName != "" {
			identifier = node.TaskName
		} else {
			return "", fmt.Errorf("task node missing identifier")
		}
	}

	combined := identifier + ":" + baseKey
	return hashString("task:" + combined), nil
}
