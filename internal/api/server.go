package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bit2swaz/velocity-cache/internal/api/ratelimit"
	"github.com/bit2swaz/velocity-cache/internal/storage"
)

// Server exposes HTTP handlers for cache operations.
type Server struct {
	s3Client       *storage.S3Client
	uploadLimiter  *ratelimit.Limiter
	downloadExpiry time.Duration
	uploadExpiry   time.Duration
	mux            *http.ServeMux
}

// NewServer constructs a new Server instance.
func NewServer(s3Client *storage.S3Client, uploadLimiter *ratelimit.Limiter, presignExpiry time.Duration) *Server {
	if presignExpiry <= 0 {
		presignExpiry = 5 * time.Minute
	}

	srv := &Server{
		s3Client:       s3Client,
		uploadLimiter:  uploadLimiter,
		downloadExpiry: presignExpiry,
		uploadExpiry:   presignExpiry,
		mux:            http.NewServeMux(),
	}

	srv.mux.HandleFunc("/api/v1/cache/download", srv.wrap(srv.handleDownload))
	srv.mux.HandleFunc("/api/v1/cache/upload", srv.wrap(srv.handleUpload))

	if uploadLimiter != nil {
		go srv.startLimiterJanitor(uploadLimiter, 5*time.Minute)
	}

	return srv
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

type presignResponse struct {
	URL              string `json:"url"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return methodNotAllowedError{allow: http.MethodGet}
	}

	cacheKey := strings.TrimSpace(r.URL.Query().Get("key"))
	if cacheKey == "" {
		return clientError{msg: "missing required query param: key", code: http.StatusBadRequest}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	exists, err := s.s3Client.CheckRemote(ctx, cacheKey)
	if err != nil {
		return serverError{err: err}
	}

	if !exists {
		return clientError{msg: "cache key not found", code: http.StatusNotFound}
	}

	url, err := s.s3Client.GenerateDownloadURL(ctx, cacheKey, s.downloadExpiry)
	if err != nil {
		return serverError{err: err}
	}

	respondJSON(w, http.StatusOK, presignResponse{
		URL:              url,
		ExpiresInSeconds: int(s.downloadExpiry.Seconds()),
	})
	return nil
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return methodNotAllowedError{allow: http.MethodPost}
	}

	cacheKey := strings.TrimSpace(r.URL.Query().Get("key"))
	if cacheKey == "" {
		return clientError{msg: "missing required query param: key", code: http.StatusBadRequest}
	}

	if s.uploadLimiter != nil {
		ip := clientIP(r)
		if ok, retryAfter := s.uploadLimiter.Allow(ip); !ok {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", formatRetryAfter(retryAfter))
			}
			return clientError{msg: "rate limit exceeded", code: http.StatusTooManyRequests}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	url, err := s.s3Client.GenerateUploadURL(ctx, cacheKey, s.uploadExpiry)
	if err != nil {
		return serverError{err: err}
	}

	respondJSON(w, http.StatusOK, presignResponse{
		URL:              url,
		ExpiresInSeconds: int(s.uploadExpiry.Seconds()),
	})
	return nil
}

// wrap converts a handler that returns an error into a standard http.HandlerFunc.
func (s *Server) wrap(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			s.handleError(w, r, err)
		}
	}
}

func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var mErr methodNotAllowedError
	var cErr clientError
	var sErr serverError

	switch {
	case errors.As(err, &mErr):
		if mErr.allow != "" {
			w.Header().Set("Allow", mErr.allow)
		}
		respondJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	case errors.As(err, &cErr):
		respondJSON(w, cErr.code, errorResponse{Error: cErr.msg})
	case errors.As(err, &sErr):
		log.Printf("ERROR: %v", sErr.err)
		respondJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	default:
		log.Printf("ERROR: %v", err)
		respondJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("ERROR: encode json response: %v", err)
	}
}

func clientIP(r *http.Request) string {
	hdr := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if hdr != "" {
		parts := strings.Split(hdr, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func formatRetryAfter(d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	seconds := int(d.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

type clientError struct {
	msg  string
	code int
}

func (e clientError) Error() string { return e.msg }

type serverError struct {
	err error
}

func (e serverError) Error() string { return e.err.Error() }

type methodNotAllowedError struct {
	allow string
}

func (methodNotAllowedError) Error() string { return "method not allowed" }

func (s *Server) startLimiterJanitor(limiter *ratelimit.Limiter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		limiter.Cleanup()
	}
}
