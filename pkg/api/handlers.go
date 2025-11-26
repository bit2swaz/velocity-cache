package api

import (
	"encoding/json"
	"net/http"

	"github.com/bit2swaz/velocity-cache/pkg/storage"
)

type NegotiateRequest struct {
	Hash   string `json:"hash"`
	Action string `json:"action"`
}

type NegotiateResponse struct {
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
}

type Handler struct {
	store storage.Driver
}

func NewHandler(store storage.Driver) *Handler {
	return &Handler{store: store}
}

func (h *Handler) HandleNegotiate(w http.ResponseWriter, r *http.Request) {
	var req NegotiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch req.Action {
	case "upload":
		exists, err := h.store.Exists(ctx, req.Hash)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if exists {
			respondJSON(w, http.StatusOK, NegotiateResponse{Status: "skipped"})
			return
		}

		url, err := h.store.GetUploadURL(ctx, req.Hash)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		respondJSON(w, http.StatusOK, NegotiateResponse{Status: "upload_needed", URL: url})

	case "download":
		exists, err := h.store.Exists(ctx, req.Hash)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !exists {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		url, err := h.store.GetDownloadURL(ctx, req.Hash)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		respondJSON(w, http.StatusOK, NegotiateResponse{Status: "found", URL: url})

	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
	}
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
