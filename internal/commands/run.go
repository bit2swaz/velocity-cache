package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bit2swaz/velocity-cache/internal/config"
	"github.com/bit2swaz/velocity-cache/internal/engine"
	"github.com/bit2swaz/velocity-cache/internal/storage"
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
	cmd := &cobra.Command{
		Use:   "run <script-name>",
		Short: "Execute a velocity script with caching",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runScript(cmd, args[0])
		},
	}
	return cmd
}

func runScript(cmd *cobra.Command, scriptName string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	start := time.Now()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	script, ok := cfg.Scripts[scriptName]
	if !ok {
		return fmt.Errorf("script %q not found in velocity.config.json", scriptName)
	}

	cacheKey, err := engine.GenerateCacheKey(script)
	if err != nil {
		return fmt.Errorf("generate cache key: %w", err)
	}

	savedDuration, hasSavedDuration := loadCacheDuration(cacheKey)

	cacheZip, found, err := engine.CheckLocal(cacheKey)
	if err != nil {
		return fmt.Errorf("check local cache: %w", err)
	}
	if found {
		if err := engine.Extract(cacheZip, script.Outputs); err != nil {
			return fmt.Errorf("extract local cache: %w", err)
		}
		logCacheHit(out, "local", time.Since(start), savedDuration, hasSavedDuration)
		return nil
	}

	var (
		s3Client          *storage.S3Client
		usePublicCacheAPI bool
		publicAPIBase     = publicAPIBaseURL()
		httpClient        = &http.Client{Timeout: 30 * time.Second}
	)

	if cfg.RemoteCache.Enabled {
		hasKeys := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")) != "" || strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID")) != ""
		if hasKeys {
			logInfo(out, "Using configured S3/R2 credentials for remote cache...")
			s3Client, err = storage.NewS3Client(ctx, cfg.RemoteCache.Bucket)
			if err != nil {
				return fmt.Errorf("create remote cache client: %w", err)
			}
		} else {
			usePublicCacheAPI = true
			logInfo(out, "No S3/R2 credentials detected. Falling back to the community cache service.")
			logWarning(errOut, "Artifacts upload anonymously. Set S3/R2 environment variables to use a private cache.")
		}
	}

	if cfg.RemoteCache.Enabled && usePublicCacheAPI {
		url, status, err := fetchPublicDownloadURL(ctx, httpClient, publicAPIBase, cacheKey)
		if err != nil {
			if !errors.Is(err, errPublicCacheMiss) {
				return fmt.Errorf("public cache download: %w", err)
			}
		} else if status == http.StatusOK && url != "" {
			tempDir, err := os.MkdirTemp("", "velocity-remote-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer os.RemoveAll(tempDir)

			tempZip := filepath.Join(tempDir, cacheKey+".zip")
			if err := downloadToFile(ctx, httpClient, url, tempZip); err != nil {
				return fmt.Errorf("download public cache: %w", err)
			}

			localZip, err := engine.SaveLocal(cacheKey, tempZip)
			if err != nil {
				return fmt.Errorf("save public cache locally: %w", err)
			}

			if err := engine.Extract(localZip, script.Outputs); err != nil {
				return fmt.Errorf("extract public cache: %w", err)
			}

			logCacheHit(out, "remote", time.Since(start), savedDuration, hasSavedDuration)
			return nil
		}
	}

	if cfg.RemoteCache.Enabled && !usePublicCacheAPI && s3Client != nil {
		hasRemote, err := s3Client.CheckRemote(ctx, cacheKey)
		if err != nil {
			return fmt.Errorf("check remote cache: %w", err)
		}

		if hasRemote {
			tempDir, err := os.MkdirTemp("", "velocity-remote-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer os.RemoveAll(tempDir)

			tempZip := filepath.Join(tempDir, cacheKey+".zip")
			if err := s3Client.DownloadRemote(ctx, cacheKey, tempZip); err != nil {
				return fmt.Errorf("download remote cache: %w", err)
			}

			localZip, err := engine.SaveLocal(cacheKey, tempZip)
			if err != nil {
				return fmt.Errorf("save remote cache locally: %w", err)
			}

			if err := downloadRemoteMetadata(ctx, s3Client, cacheKey); err != nil {
				logWarning(errOut, fmt.Sprintf("failed to download cache metadata: %v", err))
			}

			savedDuration, hasSavedDuration = loadCacheDuration(cacheKey)

			if err := engine.Extract(localZip, script.Outputs); err != nil {
				return fmt.Errorf("extract remote cache: %w", err)
			}

			logCacheHit(out, "remote", time.Since(start), savedDuration, hasSavedDuration)
			return nil
		}
	}

	logCacheMissExecuting(out, script.Command)
	execStart := time.Now()
	exitCode, execErr := engine.Execute(script)
	execDuration := time.Since(execStart)
	if execErr != nil {
		logCacheFailure(errOut, script.Command, exitCode, execErr)
		return newExitError(exitCode, fmt.Errorf("execute script: %w", execErr))
	}

	tempDir, err := os.MkdirTemp("", "velocity-outputs-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempZip := filepath.Join(tempDir, cacheKey+".zip")
	if err := engine.Compress(script.Outputs, tempZip); err != nil {
		return fmt.Errorf("compress outputs: %w", err)
	}

	localZip, err := engine.SaveLocal(cacheKey, tempZip)
	if err != nil {
		return fmt.Errorf("save cache locally: %w", err)
	}

	if err := storeCacheMetadata(cacheKey, script.Command, execDuration); err != nil {
		logWarning(errOut, fmt.Sprintf("failed to record cache metadata: %v", err))
	} else {
		savedDuration, hasSavedDuration = execDuration, true
	}

	if cfg.RemoteCache.Enabled {
		if usePublicCacheAPI {
			logCacheMissUpload(out)
			uploadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			if err := uploadViaPublicAPI(uploadCtx, httpClient, publicAPIBase, cacheKey, localZip); err != nil {
				logAsyncFailure(errOut, err)
			}
			if metaPath, err := engine.LocalCacheMetadataPath(cacheKey); err == nil {
				if metaKey, err := engine.CacheMetadataObjectName(cacheKey); err == nil {
					if err := uploadViaPublicAPI(uploadCtx, httpClient, publicAPIBase, metaKey, metaPath); err != nil {
						logAsyncFailure(errOut, err)
					}
				}
			}
		} else if s3Client != nil {
			logCacheMissUpload(out)
			uploads := make([]<-chan error, 0, 2)
			uploads = append(uploads, s3Client.UploadRemote(context.Background(), cacheKey, localZip))
			if metaPath, err := engine.LocalCacheMetadataPath(cacheKey); err == nil {
				if metaKey, err := engine.CacheMetadataObjectName(cacheKey); err == nil {
					uploads = append(uploads, s3Client.UploadRemote(context.Background(), metaKey, metaPath))
				}
			}
			for _, ch := range uploads {
				if err := <-ch; err != nil {
					logAsyncFailure(errOut, err)
				}
			}
		}
	}

	logCacheStored(out, cacheKey, execDuration, savedDuration, hasSavedDuration)
	return nil
}

func prefix() string {
	return prefixStyle.Sprint("[VelocityCache]")
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

const defaultPublicAPIBase = "https://velocity-api-2pno.onrender.com"

var errPublicCacheMiss = errors.New("public cache miss")

func publicAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("VELOCITY_PUBLIC_CACHE_API")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return defaultPublicAPIBase
}

func fetchPublicDownloadURL(ctx context.Context, client *http.Client, base, cacheKey string) (string, int, error) {
	endpoint, err := buildPublicAPIURL(base, "/api/v1/cache/download", cacheKey)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", resp.StatusCode, errPublicCacheMiss
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", resp.StatusCode, fmt.Errorf("public cache download: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", resp.StatusCode, fmt.Errorf("decode download response: %w", err)
	}
	if payload.URL == "" {
		return "", resp.StatusCode, fmt.Errorf("download response missing url")
	}
	return payload.URL, resp.StatusCode, nil
}

func uploadViaPublicAPI(ctx context.Context, client *http.Client, base, cacheKey, path string) error {
	endpoint, err := buildPublicAPIURL(base, "/api/v1/cache/upload", cacheKey)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("public cache upload: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode upload response: %w", err)
	}
	if payload.URL == "" {
		return fmt.Errorf("upload response missing url")
	}

	return uploadToPresignedURL(ctx, client, payload.URL, path)
}

func buildPublicAPIURL(base, path, cacheKey string) (string, error) {
	if base == "" {
		base = defaultPublicAPIBase
	}
	base = strings.TrimSuffix(base, "/")
	full := base + path
	parsed, err := url.Parse(full)
	if err != nil {
		return "", err
	}
	q := parsed.Query()
	q.Set("key", cacheKey)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
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
