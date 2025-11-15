package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lucsky/cuid"

	"github.com/bit2swaz/velocity-cache/internal/api/ratelimit"
	"github.com/bit2swaz/velocity-cache/internal/storage"
)

// Server exposes HTTP handlers for cache operations.
type Server struct {
	db            *pgxpool.Pool
	s3Client      *storage.S3Client
	uploadLimiter *ratelimit.Limiter
	presignExpiry time.Duration
	router        chi.Router
}

type CacheEventRequest struct {
	ProjectID string `json:"projectId"`
	Hash      string `json:"hash"`
	Status    string `json:"status"`
	Size      int    `json:"size"`
	Duration  int    `json:"duration"`
}

type PresignResponse struct {
	URL     string `json:"url"`
	Warning string `json:"warning,omitempty"`
}

// NewServer constructs a new Server instance.
func NewServer(db *pgxpool.Pool, s3Client *storage.S3Client, uploadLimiter *ratelimit.Limiter, presignExpiry time.Duration) *Server {
	if presignExpiry <= 0 {
		presignExpiry = 5 * time.Minute
	}

	srv := &Server{
		db:            db,
		s3Client:      s3Client,
		uploadLimiter: uploadLimiter,
		presignExpiry: presignExpiry,
	}

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/cache", func(r chi.Router) {
			r.Use(srv.AuthMiddleware)
			r.Post("/upload", srv.HandleUpload)
			r.Get("/download", srv.HandleDownload)
			r.Post("/event", srv.HandleCacheEvent)
		})
	})

	srv.router = router

	if uploadLimiter != nil {
		go srv.startLimiterJanitor(uploadLimiter, 5*time.Minute)
	}

	return srv
}

type contextKey string

const UserIDKey contextKey = "user_id"

// AuthMiddleware validates bearer tokens against stored API keys.
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		hash := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(hash[:])

		var userID string
		if err := s.db.QueryRow(context.Background(), "SELECT \"userId\" FROM \"ApiToken\" WHERE \"tokenHash\" = $1", tokenHash).Scan(&userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			log.Printf("ERROR: auth lookup failed: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) HandleCacheEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.db == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, _ := r.Context().Value(UserIDKey).(string)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body := http.MaxBytesReader(w, r.Body, 1<<20)
	defer body.Close()

	var req CacheEventRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Hash = strings.TrimSpace(req.Hash)
	req.Status = strings.TrimSpace(req.Status)

	if req.ProjectID == "" {
		http.Error(w, "projectId is required", http.StatusBadRequest)
		return
	}
	if req.Hash == "" {
		http.Error(w, "hash is required", http.StatusBadRequest)
		return
	}
	if req.Status == "" {
		http.Error(w, "status is required", http.StatusBadRequest)
		return
	}
	if req.Size < 0 {
		http.Error(w, "size must be zero or positive", http.StatusBadRequest)
		return
	}
	if req.Duration < 0 {
		http.Error(w, "duration must be zero or positive", http.StatusBadRequest)
		return
	}

	const authQuery = "SELECT T1.\"orgId\" FROM \"Project\" AS T1 JOIN \"OrgMember\" AS T2 ON T1.\"orgId\" = T2.\"orgId\" WHERE T1.id = $1 AND T2.\"userId\" = $2"
	var orgID string
	err := s.db.QueryRow(r.Context(), authQuery, req.ProjectID, userID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err != nil {
		log.Printf("ERROR: authorize cache event user %s project %s: %v", userID, req.ProjectID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	const insertQuery = "INSERT INTO \"CacheEvent\" (id, \"createdAt\", status, hash, size, duration, \"projectId\") VALUES ($1, NOW(), $2, $3, $4, $5, $6)"
	eventID := cuid.New()
	if _, err := s.db.Exec(r.Context(), insertQuery, eventID, req.Status, req.Hash, req.Size, req.Duration, req.ProjectID); err != nil {
		log.Printf("ERROR: insert cache event user %s project %s: %v", userID, req.ProjectID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId := r.Context().Value(UserIDKey).(string)
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	projectId := strings.TrimSpace(r.URL.Query().Get("projectId"))

	if key == "" {
		http.Error(w, "missing required query param: key", http.StatusBadRequest)
		return
	}
	if projectId == "" {
		http.Error(w, "missing required query param: projectId", http.StatusBadRequest)
		return
	}

	if s.uploadLimiter != nil {
		ip := clientIP(r)
		if ok, retryAfter := s.uploadLimiter.Allow(ip); !ok {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", formatRetryAfter(retryAfter))
			}
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	}

	var orgId string
	err := s.db.QueryRow(r.Context(), "SELECT T1.\"orgId\" FROM \"Project\" AS T1 JOIN \"OrgMember\" AS T2 ON T1.\"orgId\" = T2.\"orgId\" WHERE T1.id = $1 AND T2.\"userId\" = $2", projectId, userId).Scan(&orgId)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err != nil {
		log.Printf("ERROR: authorize upload user %s project %s: %v", userId, projectId, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	warning, err := s.evaluateQuota(r.Context(), orgId)
	if err != nil {
		log.Printf("ERROR: quota check user %s org %s: %v", userId, orgId, err)
	}

	// TODO: Implement quota check here.

	objectKey := fmt.Sprintf("%s/%s/%s.zip", orgId, projectId, key)

	url, err := s.s3Client.GeneratePresignedUploadURL(objectKey, s.presignExpiry)
	if err != nil {
		log.Printf("ERROR: generate upload URL for %s: %v", objectKey, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, PresignResponse{URL: url, Warning: warning})
}

func (s *Server) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId := r.Context().Value(UserIDKey).(string)
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	projectId := strings.TrimSpace(r.URL.Query().Get("projectId"))

	if key == "" {
		http.Error(w, "missing required query param: key", http.StatusBadRequest)
		return
	}
	if projectId == "" {
		http.Error(w, "missing required query param: projectId", http.StatusBadRequest)
		return
	}

	var orgId string
	err := s.db.QueryRow(r.Context(), "SELECT T1.\"orgId\" FROM \"Project\" AS T1 JOIN \"OrgMember\" AS T2 ON T1.\"orgId\" = T2.\"orgId\" WHERE T1.id = $1 AND T2.\"userId\" = $2", projectId, userId).Scan(&orgId)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err != nil {
		log.Printf("ERROR: authorize download user %s project %s: %v", userId, projectId, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	warning, err := s.evaluateQuota(r.Context(), orgId)
	if err != nil {
		log.Printf("ERROR: quota check user %s org %s: %v", userId, orgId, err)
	}

	// TODO: Implement quota check here.

	objectKey := fmt.Sprintf("%s/%s/%s.zip", orgId, projectId, key)

	url, err := s.s3Client.GeneratePresignedDownloadURL(objectKey, s.presignExpiry)
	if err != nil {
		log.Printf("ERROR: generate download URL for %s: %v", objectKey, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, PresignResponse{URL: url, Warning: warning})
}

func (s *Server) evaluateQuota(ctx context.Context, orgID string) (string, error) {
	var plan string
	if err := s.db.QueryRow(ctx, "SELECT plan FROM \"Organization\" WHERE id = $1", orgID).Scan(&plan); err != nil {
		return "", err
	}

	if plan != "free" {
		return "", nil
	}

	const usageQuery = "SELECT COALESCE(SUM(size), 0) FROM \"CacheEvent\" WHERE \"projectId\" IN (SELECT id FROM \"Project\" WHERE \"orgId\" = $1)"
	var currentUsage int
	if err := s.db.QueryRow(ctx, usageQuery, orgID).Scan(&currentUsage); err != nil {
		return "", err
	}

	const freeQuota = 2 * 1024 * 1024 * 1024 // 2GB
	if currentUsage <= freeQuota {
		return "", nil
	}

	usedGB := float64(currentUsage) / float64(1024*1024*1024)
	warning := fmt.Sprintf("WARNING: Your org has used %.1f/2.0GB of free cache. Join the Pro waitlist at https://velocitycache.dev/billing", usedGB)
	return warning, nil
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
