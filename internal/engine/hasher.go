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
func GenerateCacheKey(cfg config.ScriptConfig) (string, error) {
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

	files, err := collectInputFiles(cfg.Inputs)
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

	finalString := strings.Join(parts, "|")

	return hashString(finalString), nil
}

func collectInputFiles(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
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
			cleaned := filepath.Clean(match)

			info, err := os.Stat(cleaned)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("stat %q: %w", cleaned, err)
			}

			if info.IsDir() {
				continue
			}

			if matcher != nil && matcher.MatchesPath(cleaned) {
				continue
			}

			if _, ok := seen[cleaned]; ok {
				continue
			}

			seen[cleaned] = struct{}{}
			files = append(files, cleaned)
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
