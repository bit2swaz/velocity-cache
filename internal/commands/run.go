package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bit2swaz/velocity-cache/internal/config"
	"github.com/bit2swaz/velocity-cache/internal/engine"
)

var (
	prefixStyle = color.New(color.FgHiCyan, color.Bold)
	hitStyle    = color.New(color.FgHiGreen, color.Bold)
	missStyle   = color.New(color.FgHiYellow, color.Bold)
	infoStyle   = color.New(color.FgHiWhite)
	subtleStyle = color.New(color.FgHiBlack)
	warnStyle   = color.New(color.FgHiMagenta, color.Bold)
	errorStyle  = color.New(color.FgHiRed, color.Bold)
)

// ExitError is an error that carries a specific exit code.
type ExitError interface {
	error
	ExitCode() int
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit code %d", e.code)
}

func (e *exitError) Unwrap() error {
	return e.err
}

func (e *exitError) ExitCode() int {
	return e.code
}

func newExitError(code int, err error) ExitError {
	if code == 0 {
		code = 1
	}
	return &exitError{code: code, err: err}
}

func newRunCommand() *cobra.Command {
	var packageSelector string
	cmd := &cobra.Command{
		Use:   "run <task-name>",
		Short: "Execute a pipeline task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// cmd.SilenceUsage = true
			return runScript(cmd, args[0], packageSelector)
		},
	}
	cmd.Flags().StringVarP(&packageSelector, "package", "p", "", "Target package")
	return cmd
}

func runScript(cmd *cobra.Command, taskName, packageSelector string) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	// 1. Load Config (YAML)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Discover Packages
	packageGlobs := []string{"apps/*", "libs/*", "packages/*"}
	if len(cfg.Packages) > 0 {
		packageGlobs = cfg.Packages
	}

	packages, err := engine.DiscoverPackages(packageGlobs)
	if err != nil {
		return fmt.Errorf("discover packages: %w", err)
	}

	if len(packages) > 0 {
		if err := engine.BuildPackageGraph(packages); err != nil {
			return fmt.Errorf("build package graph: %w", err)
		}
	}

	// 3. Select Target
	target, err := selectTargetPackage(packageSelector, packages)
	if err != nil {
		return err
	}

	// 4. Build Graph
	// Note: cfg is passed to look up tasks in the Pipeline
	root, err := engine.BuildTaskGraph(taskName, target, packages, cfg, nil)
	if err != nil {
		return fmt.Errorf("build task graph: %w", err)
	}

	// 5. Execute
	exec := &Engine{
		ctx:    ctx,
		cfg:    cfg,
		out:    out,
		errOut: cmd.ErrOrStderr(),
	}

	// Initialize Remote Client if enabled in YAML
	if cfg.Remote.Enabled {
		// V3: No more S3 keys check. We just use the configured URL/Token.
		exec.remote = engine.NewRemoteClient(cfg.Remote.URL, cfg.Remote.Token)
	}

	_, err = exec.ExecuteTask(root)
	return err
}

type Engine struct {
	ctx    context.Context
	cfg    *config.Config
	out    io.Writer
	errOut io.Writer
	remote *engine.RemoteClient
}

func (e *Engine) ExecuteTask(task *engine.TaskNode) (string, error) {
	if task == nil {
		return "", nil
	}

	// Cycle/State checks
	if task.State == 2 {
		return task.CacheKey, nil
	}
	if task.State == 1 {
		return "", fmt.Errorf("cycle detected while executing %s", task.ID)
	}
	task.State = 1

	logTaskHeader(e.out, task.ID)

	// 1. Run Dependencies (Parallel)
	var wg sync.WaitGroup
	var depKeys []string
	var depMu sync.Mutex
	var depErr error

	for _, dep := range task.Dependencies {
		wg.Add(1)
		go func(d *engine.TaskNode) {
			defer wg.Done()
			k, err := e.ExecuteTask(d)
			depMu.Lock()
			if err != nil && depErr == nil {
				depErr = err
			}
			if k != "" {
				depKeys = append(depKeys, k)
			}
			depMu.Unlock()
		}(dep)
	}
	wg.Wait()
	if depErr != nil {
		task.State = 3
		return "", depErr
	}

	// 2. Generate Hash
	key, err := engine.GenerateTaskNodeCacheKey(task, depKeys)
	if err != nil {
		return "", err
	}
	task.CacheKey = key

	start := time.Now()
	packagePath := ""
	if task.Package != nil {
		packagePath = task.Package.Path
	}

	// 3. Check Local Cache
	cacheZip, found, err := engine.CheckLocal(key)
	if err == nil && found {
		if err := engine.Extract(cacheZip, task.TaskConfig.Outputs, packagePath); err == nil {
			logCacheHit(e.out, "local", time.Since(start))
			task.State = 2
			return key, nil
		}
	}

	// 4. Check Remote Cache (V3 Negotiation)
	if e.remote != nil {
		resp, err := e.remote.Negotiate(e.ctx, key, "download")
		if err == nil && resp.Status == "found" {
			// HIT! Download it.
			tmp, _ := os.CreateTemp("", "velo-dl-*.zip")
			defer os.Remove(tmp.Name())

			// V3 Transfer Agent handles S3 vs Proxy logic internally
			err = engine.Transfer(e.ctx, "GET", resp.URL, e.cfg.Remote.URL, nil, tmp, 0, e.cfg.Remote.Token)
			if err == nil {
				tmp.Close()
				// Save to local cache for next time
				localZip, _ := engine.SaveLocal(key, tmp.Name())
				engine.Extract(localZip, task.TaskConfig.Outputs, packagePath)

				logCacheHit(e.out, "remote", time.Since(start))
				task.State = 2
				return key, nil
			}
		}
	}

	// 5. Execute Task (Cache Miss)
	logCacheMissExecuting(e.out, task.TaskConfig.Command)
	if _, err := engine.Execute(task.TaskConfig, packagePath); err != nil {
		task.State = 3
		return "", err
	}

	// 6. Upload Cache (V3 Negotiation)
	// We only attempt upload if remote is enabled
	if e.remote != nil {
		resp, err := e.remote.Negotiate(e.ctx, key, "upload")
		if err == nil && resp.Status == "upload_needed" {
			logInfo(e.out, "Uploading artifact...")

			// Compress
			tmp, _ := os.CreateTemp("", "velo-up-*.zip")
			defer os.Remove(tmp.Name())
			engine.Compress(task.TaskConfig.Outputs, tmp.Name(), packagePath)

			// Save to local cache first (so we have the file to upload)
			localZip, _ := engine.SaveLocal(key, tmp.Name())

			// Transfer
			f, _ := os.Open(localZip)
			stat, _ := f.Stat()
			err = engine.Transfer(e.ctx, "PUT", resp.URL, e.cfg.Remote.URL, f, nil, stat.Size(), e.cfg.Remote.Token)
			f.Close()

			if err != nil {
				logWarning(e.errOut, fmt.Sprintf("Upload failed: %v", err))
			} else {
				logInfo(e.out, "Upload complete.")
			}
		} else if resp != nil && resp.Status == "skipped" {
			logInfo(e.out, "Artifact already exists remotely (skipped).")
		}
	} else {
		// If remote is disabled, just save local
		tmp, _ := os.CreateTemp("", "velo-local-*.zip")
		defer os.Remove(tmp.Name())
		engine.Compress(task.TaskConfig.Outputs, tmp.Name(), packagePath)
		engine.SaveLocal(key, tmp.Name())
	}

	task.State = 2
	return key, nil
}

// --- Helper Functions (Kept from your previous code) ---

func selectTargetPackage(selector string, packages map[string]*engine.Package) (*engine.Package, error) {
	if len(packages) == 0 {
		root := &engine.Package{
			Name:            "__workspace__",
			Path:            ".",
			PackageJsonPath: "",
		}
		packages[root.Name] = root
		return root, nil
	}

	trimmed := strings.TrimSpace(selector)
	if trimmed != "" {
		if pkg, ok := packages[trimmed]; ok {
			return pkg, nil
		}
		for _, pkg := range packages {
			if pkg.Path == trimmed {
				return pkg, nil
			}
		}
		return nil, fmt.Errorf("package %q not found. available: %s", trimmed, strings.Join(availablePackageDescriptions(packages), ", "))
	}

	if len(packages) == 1 {
		for _, pkg := range packages {
			return pkg, nil
		}
	}

	roots := rootPackages(packages)
	if len(roots) == 1 {
		return roots[0], nil
	}

	if len(roots) > 1 {
		descriptions := packageSliceDescriptions(roots)
		return nil, fmt.Errorf("multiple candidate packages found (%s). specify --package to choose one", strings.Join(descriptions, ", "))
	}

	return nil, fmt.Errorf("unable to determine target package. specify --package. available: %s", strings.Join(availablePackageDescriptions(packages), ", "))
}

func rootPackages(packages map[string]*engine.Package) []*engine.Package {
	depSet := make(map[string]struct{})
	for _, pkg := range packages {
		for _, depName := range pkg.InternalDepNames {
			depSet[depName] = struct{}{}
		}
	}
	roots := make([]*engine.Package, 0, len(packages))
	for name, pkg := range packages {
		if _, ok := depSet[name]; !ok {
			roots = append(roots, pkg)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Name < roots[j].Name
	})
	return roots
}

func availablePackageDescriptions(packages map[string]*engine.Package) []string {
	desc := make([]string, 0, len(packages))
	for _, pkg := range packages {
		desc = append(desc, pkg.Name)
	}
	sort.Strings(desc)
	return desc
}

func packageSliceDescriptions(pkgs []*engine.Package) []string {
	desc := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		desc = append(desc, pkg.Name)
	}
	return desc
}

// Logging helpers
func prefix() string { return prefixStyle.Sprint("[VelocityCache]") }

func logTaskHeader(out io.Writer, nodeID string) {
	fmt.Fprintf(out, "%s %s\n", prefix(), infoStyle.Sprintf("Task %s", nodeID))
}

func logCacheHit(out io.Writer, scope string, elapsed time.Duration) {
	fmt.Fprintf(out, "%s %s in %s\n", prefix(), hitStyle.Sprintf("CACHE HIT (%s)", scope), elapsed.Round(time.Millisecond))
}

func logCacheMissExecuting(out io.Writer, command string) {
	fmt.Fprintf(out, "%s %s %s\n", prefix(), missStyle.Sprint("CACHE MISS."), infoStyle.Sprintf("Executing %q...", command))
}

func logInfo(out io.Writer, message string) {
	fmt.Fprintf(out, "%s %s\n", prefix(), infoStyle.Sprint(message))
}

func logWarning(errOut io.Writer, message string) {
	fmt.Fprintf(errOut, "%s %s %s\n", prefix(), warnStyle.Sprint("WARN"), infoStyle.Sprint(message))
}
