package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bit2swaz/velocity-cache/internal/auth"
	"github.com/bit2swaz/velocity-cache/internal/config"
	"github.com/bit2swaz/velocity-cache/internal/engine"
	"github.com/bit2swaz/velocity-cache/internal/storage"
)

var (
	prefixStyle        = color.New(color.FgHiCyan, color.Bold)
	hitStyle           = color.New(color.FgHiGreen, color.Bold)
	missStyle          = color.New(color.FgHiYellow, color.Bold)
	infoStyle          = color.New(color.FgHiWhite)
	subtleStyle        = color.New(color.FgHiBlack)
	warnStyle          = color.New(color.FgHiMagenta, color.Bold)
	errorStyle         = color.New(color.FgHiRed, color.Bold)
	errPublicCacheMiss = engine.ErrPublicCacheMiss
)

type cacheMetadata struct {
	Command        string    `json:"command"`
	DurationMillis int64     `json:"duration_millis"`
	RecordedAt     time.Time `json:"recorded_at"`
}

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
		Use:   "run <script-name>",
		Short: "Execute a velocity script with caching",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runScript(cmd, args[0], packageSelector)
		},
	}

	cmd.Flags().StringVarP(&packageSelector, "package", "p", "", "Target package name or path to run the task against")

	return cmd
}

func runScript(cmd *cobra.Command, scriptName, packageSelector string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if _, ok := cfg.Tasks[scriptName]; !ok {
		return fmt.Errorf("task %q not found in velocity.config.json", scriptName)
	}

	packages, err := engine.DiscoverPackages(cfg.Packages)
	if err != nil {
		return fmt.Errorf("discover packages: %w", err)
	}

	if len(packages) > 0 {
		if err := engine.BuildPackageGraph(packages); err != nil {
			return fmt.Errorf("build package graph: %w", err)
		}
	}

	targetPackage, err := selectTargetPackage(packageSelector, packages)
	if err != nil {
		return err
	}

	rootNode, err := engine.BuildTaskGraph(scriptName, targetPackage, packages, cfg, map[string]bool{})
	if err != nil {
		return fmt.Errorf("build task graph: %w", err)
	}

	if err := ExecuteGraph(ctx, rootNode, cfg, out, errOut); err != nil {
		return err
	}

	return nil
}

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
		if roots[i].Path == roots[j].Path {
			return roots[i].Name < roots[j].Name
		}
		return roots[i].Path < roots[j].Path
	})

	return roots
}

func availablePackageDescriptions(packages map[string]*engine.Package) []string {
	desc := make([]string, 0, len(packages))
	for _, pkg := range packages {
		desc = append(desc, describePackage(pkg))
	}
	sort.Strings(desc)
	return desc
}

func packageSliceDescriptions(pkgs []*engine.Package) []string {
	desc := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		desc = append(desc, describePackage(pkg))
	}
	return desc
}

func describePackage(pkg *engine.Package) string {
	if pkg == nil {
		return "<unknown>"
	}

	name := strings.TrimSpace(pkg.Name)
	path := strings.TrimSpace(pkg.Path)

	switch {
	case name != "" && path != "" && name != path:
		return fmt.Sprintf("%s (%s)", name, path)
	case path != "":
		return path
	case name != "":
		return name
	default:
		return "<unnamed>"
	}
}

// ExecuteGraph walks the task dependency graph, executing each node exactly once
// after all of its dependencies have completed (or been restored from cache).

func ExecuteGraph(ctx context.Context, root *engine.TaskNode, cfg *config.Config, out, errOut io.Writer) error {
	if root == nil {
		return nil
	}

	executor, err := newEngine(ctx, cfg, out, errOut)
	if err != nil {
		return err
	}

	_, err = executor.ExecuteTask(root)
	return err
}

type Engine struct {
	ctx           context.Context
	cfg           *config.Config
	out           io.Writer
	errOut        io.Writer
	httpClient    *http.Client
	s3Client      *storage.S3Client
	useSaaS       bool
	apiToken      string
	projectId     string
	publicAPIBase string
}

func newEngine(ctx context.Context, cfg *config.Config, out, errOut io.Writer) (*Engine, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	token, _ := auth.LoadToken()
	token = strings.TrimSpace(token)
	projectID := strings.TrimSpace(cfg.ProjectID)
	hasKeys := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")) != "" || strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID")) != ""

	exec := &Engine{
		ctx:           ctx,
		cfg:           cfg,
		out:           out,
		errOut:        errOut,
		publicAPIBase: publicAPIBaseURL(),
	}

	if cfg.RemoteCache.Enabled {
		exec.httpClient = &http.Client{Timeout: 30 * time.Second}

		if token != "" && projectID != "" {
			logInfo(out, "Using VelocityCache SaaS account.")
			exec.useSaaS = true
			exec.apiToken = token
			exec.projectId = projectID
		} else if hasKeys {
			logInfo(out, "Using private S3/R2 credentials.")
			client, err := storage.NewS3Client(ctx, cfg.RemoteCache.Bucket)
			if err != nil {
				return nil, fmt.Errorf("create remote cache client: %w", err)
			}
			exec.s3Client = client
		} else {
			logInfo(out, "No credentials found. Using local cache only.")
		}
	}

	return exec, nil
}

func (e *Engine) ExecuteTask(task *engine.TaskNode) (string, error) {
	if task == nil {
		return "", nil
	}

	switch task.State {
	case 2:
		return task.CacheKey, nil
	case 1:
		return "", fmt.Errorf("cycle detected while executing %s", task.ID)
	case 3:
		if task.LastError != nil {
			return "", task.LastError
		}
		return "", fmt.Errorf("task %s previously failed", task.ID)
	}

	task.State = 1
	task.CacheKey = ""
	task.LastError = nil

	logTaskHeader(e.out, task.ID)

	depKeys := make([]string, 0, len(task.Dependencies))
	if len(task.Dependencies) > 0 {
		var (
			depMu  sync.Mutex
			depErr error
			wg     sync.WaitGroup
		)

		for _, dep := range task.Dependencies {
			dep := dep
			if dep == nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				key, err := e.ExecuteTask(dep)
				if err != nil {
					depMu.Lock()
					if depErr == nil {
						depErr = err
					}
					depMu.Unlock()
					return
				}
				if key != "" {
					depMu.Lock()
					depKeys = append(depKeys, key)
					depMu.Unlock()
				}
			}()
		}
		wg.Wait()
		if depErr != nil {
			task.State = 3
			task.LastError = depErr
			return "", depErr
		}
	}

	cacheKey, err := engine.GenerateTaskNodeCacheKey(task, depKeys)
	if err != nil {
		task.State = 3
		err = fmt.Errorf("generate cache key for %s: %w", task.ID, err)
		task.LastError = err
		return "", err
	}

	task.CacheKey = cacheKey

	savedDuration, hasSavedDuration := loadCacheDuration(cacheKey)
	start := time.Now()
	taskCfg := task.TaskConfig
	packagePath := ""
	if task.Package != nil {
		packagePath = task.Package.Path
	}

	cacheZip, found, err := engine.CheckLocal(cacheKey)
	if err != nil {
		task.State = 3
		err = fmt.Errorf("check local cache for %s: %w", task.ID, err)
		task.LastError = err
		return "", err
	}
	if found {
		if err := engine.Extract(cacheZip, taskCfg.Outputs, packagePath); err != nil {
			task.State = 3
			err = fmt.Errorf("extract local cache for %s: %w", task.ID, err)
			task.LastError = err
			return "", err
		}
		logCacheHit(e.out, "local", time.Since(start), savedDuration, hasSavedDuration)
		go reportCacheEvent(e, task, "HIT", 0, time.Since(start))
		task.State = 2
		return cacheKey, nil
	}

	if e.cfg.RemoteCache.Enabled {
		if e.useSaaS {
			apiClient := engine.NewSaaSAPIClient(e.publicAPIBase, e.apiToken)
			apiClient.SetHTTPClient(e.httpClient)

			downloadResp, found, err := apiClient.GetDownloadURL(e.ctx, e.projectId, cacheKey)
			if err != nil {
				if errors.Is(err, errPublicCacheMiss) {
					found = false
				} else {
					task.State = 3
					err = fmt.Errorf("velocity SaaS download for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}
			}
			if found {
				if strings.TrimSpace(downloadResp.Warning) != "" {
					logWarning(e.errOut, downloadResp.Warning)
				}
				tempDir, err := os.MkdirTemp("", "velocity-remote-*")
				if err != nil {
					task.State = 3
					err = fmt.Errorf("create temp dir for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}
				defer os.RemoveAll(tempDir)

				tempZip := filepath.Join(tempDir, cacheKey+".zip")
				if err := downloadToFile(e.ctx, apiClient.HTTPClient(), downloadResp.URL, tempZip); err != nil {
					task.State = 3
					err = fmt.Errorf("download SaaS cache for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}

				localZip, err := engine.SaveLocal(cacheKey, tempZip)
				if err != nil {
					task.State = 3
					err = fmt.Errorf("save SaaS cache locally for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}

				if err := engine.Extract(localZip, taskCfg.Outputs, packagePath); err != nil {
					task.State = 3
					err = fmt.Errorf("extract SaaS cache for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}

				logCacheHit(e.out, "remote", time.Since(start), savedDuration, hasSavedDuration)
				go reportCacheEvent(e, task, "HIT", 0, time.Since(start))
				task.State = 2
				return cacheKey, nil
			}
		} else if e.s3Client != nil {
			hasRemote, err := e.s3Client.CheckRemote(e.ctx, cacheKey)
			if err != nil {
				task.State = 3
				err = fmt.Errorf("check remote cache for %s: %w", task.ID, err)
				task.LastError = err
				return "", err
			}

			if hasRemote {
				tempDir, err := os.MkdirTemp("", "velocity-remote-*")
				if err != nil {
					task.State = 3
					err = fmt.Errorf("create temp dir for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}
				defer os.RemoveAll(tempDir)

				tempZip := filepath.Join(tempDir, cacheKey+".zip")
				if err := e.s3Client.DownloadRemote(e.ctx, cacheKey, tempZip); err != nil {
					task.State = 3
					err = fmt.Errorf("download remote cache for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}

				localZip, err := engine.SaveLocal(cacheKey, tempZip)
				if err != nil {
					task.State = 3
					err = fmt.Errorf("save remote cache locally for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}

				if err := downloadRemoteMetadata(e.ctx, e.s3Client, cacheKey); err != nil {
					logWarning(e.errOut, fmt.Sprintf("failed to download cache metadata: %v", err))
				}

				savedDuration, hasSavedDuration = loadCacheDuration(cacheKey)

				if err := engine.Extract(localZip, taskCfg.Outputs, packagePath); err != nil {
					task.State = 3
					err = fmt.Errorf("extract remote cache for %s: %w", task.ID, err)
					task.LastError = err
					return "", err
				}

				logCacheHit(e.out, "remote", time.Since(start), savedDuration, hasSavedDuration)
				go reportCacheEvent(e, task, "HIT", 0, time.Since(start))
				task.State = 2
				return cacheKey, nil
			}
		}
	}

	logCacheMissExecuting(e.out, taskCfg.Command)
	execStart := time.Now()
	exitCode, execErr := engine.Execute(taskCfg, packagePath)
	execDuration := time.Since(execStart)
	if execErr != nil {
		logCacheFailure(e.errOut, taskCfg.Command, exitCode, execErr)
		task.State = 3
		err := newExitError(exitCode, fmt.Errorf("execute task %s: %w", task.ID, execErr))
		task.LastError = err
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "velocity-outputs-*")
	if err != nil {
		task.State = 3
		err = fmt.Errorf("create temp dir for %s: %w", task.ID, err)
		task.LastError = err
		return "", err
	}
	defer os.RemoveAll(tempDir)

	tempZip := filepath.Join(tempDir, cacheKey+".zip")
	if err := engine.Compress(taskCfg.Outputs, tempZip, packagePath); err != nil {
		task.State = 3
		err = fmt.Errorf("compress outputs for %s: %w", task.ID, err)
		task.LastError = err
		return "", err
	}

	localZip, err := engine.SaveLocal(cacheKey, tempZip)
	if err != nil {
		task.State = 3
		err = fmt.Errorf("save cache locally for %s: %w", task.ID, err)
		task.LastError = err
		return "", err
	}

	fileInfo, err := os.Stat(localZip)
	if err != nil {
		logWarning(e.errOut, fmt.Sprintf("failed to stat cache artifact: %v", err))
	} else {
		go reportCacheEvent(e, task, "MISS", int(fileInfo.Size()), execDuration)
	}

	if err := storeCacheMetadata(cacheKey, taskCfg.Command, execDuration); err != nil {
		logWarning(e.errOut, fmt.Sprintf("failed to record cache metadata: %v", err))
	} else {
		savedDuration, hasSavedDuration = execDuration, true
	}

	if e.cfg.RemoteCache.Enabled {
		if e.useSaaS {
			logCacheMissUpload(e.out)
			uploadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			apiClient := engine.NewSaaSAPIClient(e.publicAPIBase, e.apiToken)
			apiClient.SetHTTPClient(e.httpClient)

			uploadResp, err := apiClient.GetUploadURL(uploadCtx, e.projectId, cacheKey)
			if err != nil {
				logAsyncFailure(e.errOut, err)
			} else if uploadResp.URL != "" {
				if strings.TrimSpace(uploadResp.Warning) != "" {
					logWarning(e.errOut, uploadResp.Warning)
				}
				if err := uploadToPresignedURL(uploadCtx, apiClient.HTTPClient(), uploadResp.URL, localZip); err != nil {
					logAsyncFailure(e.errOut, err)
				}
			}

			if metaPath, err := engine.LocalCacheMetadataPath(cacheKey); err == nil {
				if metaKey, err := engine.CacheMetadataObjectName(cacheKey); err == nil {
					metaResp, err := apiClient.GetUploadURL(uploadCtx, e.projectId, metaKey)
					if err != nil {
						logAsyncFailure(e.errOut, err)
					} else if metaResp.URL != "" {
						if strings.TrimSpace(metaResp.Warning) != "" {
							logWarning(e.errOut, metaResp.Warning)
						}
						if err := uploadToPresignedURL(uploadCtx, apiClient.HTTPClient(), metaResp.URL, metaPath); err != nil {
							logAsyncFailure(e.errOut, err)
						}
					}
				}
			}
		} else if e.s3Client != nil {
			logCacheMissUpload(e.out)
			uploads := make([]<-chan error, 0, 2)
			uploads = append(uploads, e.s3Client.UploadRemote(context.Background(), cacheKey, localZip))
			if metaPath, err := engine.LocalCacheMetadataPath(cacheKey); err == nil {
				if metaKey, err := engine.CacheMetadataObjectName(cacheKey); err == nil {
					uploads = append(uploads, e.s3Client.UploadRemote(context.Background(), metaKey, metaPath))
				}
			}
			for _, ch := range uploads {
				if err := <-ch; err != nil {
					logAsyncFailure(e.errOut, err)
				}
			}
		}
	}

	logCacheStored(e.out, cacheKey, execDuration, savedDuration, hasSavedDuration)
	task.State = 2
	return cacheKey, nil
}

func prefix() string {
	return prefixStyle.Sprint("[VelocityCache]")
}

func logTaskHeader(out io.Writer, nodeID string) {
	if strings.TrimSpace(nodeID) == "" {
		nodeID = "<unnamed>"
	}
	fmt.Fprintf(out, "%s %s\n",
		prefix(),
		infoStyle.Sprintf("Task %s", nodeID),
	)
}

func logCacheHit(out io.Writer, scope string, elapsed time.Duration, saved time.Duration, hasSaved bool) {
	savedSuffix := ""
	if hasSaved && saved > 0 {
		savedSuffix = " " + subtleStyle.Sprintf("(saved %s)", humanDuration(saved))
	}
	fmt.Fprintf(out, "%s %s in %s%s\n",
		prefix(),
		hitStyle.Sprintf("CACHE HIT (%s)", scope),
		humanDuration(elapsed),
		savedSuffix,
	)
}

func logCacheMissExecuting(out io.Writer, command string) {
	fmt.Fprintf(out, "%s %s %s\n",
		prefix(),
		missStyle.Sprint("CACHE MISS."),
		infoStyle.Sprintf("Executing %q...", command),
	)
}

func logCacheMissUpload(out io.Writer) {
	fmt.Fprintf(out, "%s %s %s\n",
		prefix(),
		missStyle.Sprint("CACHE MISS."),
		infoStyle.Sprint("Uploading to remote cache (async)..."),
	)
}

func logInfo(out io.Writer, message string) {
	fmt.Fprintf(out, "%s %s\n", prefix(), infoStyle.Sprint(message))
}

func logCacheStored(out io.Writer, cacheKey string, execDuration time.Duration, saved time.Duration, hasSaved bool) {
	savings := ""
	if hasSaved && saved > 0 {
		savings = " " + subtleStyle.Sprintf("(future savings ~%s)", humanDuration(saved))
	}
	fmt.Fprintf(out, "%s %s %s%s\n",
		prefix(),
		missStyle.Sprint("CACHE MISS."),
		infoStyle.Sprintf("Stored cache %q in %s.", cacheKey, humanDuration(execDuration)),
		savings,
	)
}

func logCacheFailure(errOut io.Writer, command string, exitCode int, execErr error) {
	fmt.Fprintf(errOut, "%s %s %s (exit code %d)\n",
		prefix(),
		errorStyle.Sprint("COMMAND FAILED."),
		infoStyle.Sprintf("%v while executing %q", execErr, command),
		exitCode,
	)
}

func logWarning(errOut io.Writer, message string) {
	fmt.Fprintf(errOut, "%s %s %s\n", prefix(), warnStyle.Sprint("WARN"), infoStyle.Sprint(message))
}

func logAsyncFailure(errOut io.Writer, err error) {
	fmt.Fprintf(errOut, "%s %s %v\n", prefix(), errorStyle.Sprint("REMOTE UPLOAD FAILED."), err)
}

const defaultPublicAPIBase = "https://velocity-api-ly66.onrender.com"

func publicAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("VELOCITY_PUBLIC_CACHE_API")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return defaultPublicAPIBase
}

func downloadToFile(ctx context.Context, client *http.Client, fileURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download presigned url: status %d", resp.StatusCode)
	}

	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}

	return file.Sync()
}

func uploadToPresignedURL(ctx context.Context, client *http.Client, fileURL, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fileURL, file)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/zip")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upload presigned url: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func humanDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "0s"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", float64(d)/float64(time.Second))
	}
	minutes := d / time.Minute
	seconds := (d % time.Minute) / time.Second
	if minutes >= 60 {
		hours := minutes / time.Hour
		minutes = minutes % time.Hour
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

func loadCacheDuration(cacheKey string) (time.Duration, bool) {
	path, err := engine.LocalCacheMetadataPath(cacheKey)
	if err != nil {
		return 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var meta cacheMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return 0, false
	}
	if meta.DurationMillis <= 0 {
		return 0, false
	}
	return time.Duration(meta.DurationMillis) * time.Millisecond, true
}

func reportCacheEvent(e *Engine, task *engine.TaskNode, status string, size int, duration time.Duration) {
	go func() {
		if e == nil || !e.useSaaS {
			return
		}
		if task == nil || strings.TrimSpace(task.CacheKey) == "" {
			return
		}
		if strings.TrimSpace(e.projectId) == "" || strings.TrimSpace(e.apiToken) == "" {
			return
		}
		if e.httpClient == nil {
			return
		}

		payload := map[string]any{
			"projectId": e.projectId,
			"hash":      task.CacheKey,
			"status":    status,
			"size":      size,
			"duration":  int(duration.Milliseconds()),
		}

		body, err := json.Marshal(payload)
		if err != nil {
			logAsyncFailure(e.errOut, fmt.Errorf("marshal cache event: %w", err))
			return
		}

		ctx := e.ctx
		if ctx == nil {
			ctx = context.Background()
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.publicAPIBase+"/api/v1/cache/event", bytes.NewReader(body))
		if err != nil {
			logAsyncFailure(e.errOut, fmt.Errorf("create cache event request: %w", err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.apiToken)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			logAsyncFailure(e.errOut, fmt.Errorf("cache event request failed: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			logAsyncFailure(e.errOut, fmt.Errorf("cache event status %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
			return
		}
	}()
}

func storeCacheMetadata(cacheKey, command string, duration time.Duration) error {
	path, err := engine.LocalCacheMetadataPath(cacheKey)
	if err != nil {
		return err
	}
	durationMillis := duration.Milliseconds()
	if durationMillis == 0 && duration > 0 {
		durationMillis = 1
	}
	meta := cacheMetadata{
		Command:        command,
		DurationMillis: durationMillis,
		RecordedAt:     time.Now().UTC(),
	}
	contents, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, contents, 0o644)
}

func downloadRemoteMetadata(ctx context.Context, client *storage.S3Client, cacheKey string) error {
	metaKey, err := engine.CacheMetadataObjectName(cacheKey)
	if err != nil {
		return err
	}
	exists, err := client.CheckRemote(ctx, metaKey)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	metaPath, err := engine.LocalCacheMetadataPath(cacheKey)
	if err != nil {
		return err
	}
	return client.DownloadRemote(ctx, metaKey, metaPath)
}
