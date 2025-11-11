package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lucsky/cuid"
)

type AppState struct {
	DB *pgxpool.Pool
}

type CacheEventRequest struct {
	ProjectID string `json:"projectId"`
	Hash      string `json:"hash"`
	Status    string `json:"status"`
	Size      int    `json:"size"`
	Duration  int    `json:"duration"`
}

func HandleCacheEvent(appState *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if appState == nil || appState.DB == nil {
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

		const authQuery = "SELECT T1.org_id FROM Project AS T1 JOIN OrgMember AS T2 ON T1.org_id = T2.org_id WHERE T1.id = $1 AND T2.user_id = $2"
		var orgID string
		err := appState.DB.QueryRow(r.Context(), authQuery, req.ProjectID, userID).Scan(&orgID)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err != nil {
			log.Printf("ERROR: authorize cache event user %s project %s: %v", userID, req.ProjectID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		const insertQuery = "INSERT INTO CacheEvent (id, \"createdAt\", status, hash, size, duration, \"projectId\") VALUES ($1, NOW(), $2, $3, $4, $5, $6)"
		eventID := cuid.New()
		if _, err := appState.DB.Exec(r.Context(), insertQuery, eventID, req.Status, req.Hash, req.Size, req.Duration, req.ProjectID); err != nil {
			log.Printf("ERROR: insert cache event user %s project %s: %v", userID, req.ProjectID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
