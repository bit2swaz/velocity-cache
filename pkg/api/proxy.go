package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/bit2swaz/velocity-cache/pkg/observability"
)

func (h *Handler) HandleProxyUpload(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	root := os.Getenv("VC_LOCAL_ROOT")
	if root == "" {
		http.Error(w, "Server configuration error: VC_LOCAL_ROOT not set", http.StatusInternalServerError)
		return
	}

	path := filepath.Join(root, key)

	out, err := os.Create(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create file: %v", err), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	n, err := io.Copy(out, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
		return
	}

	observability.ProxyTraffic.WithLabelValues("in").Add(float64(n))

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleProxyDownload(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	root := os.Getenv("VC_LOCAL_ROOT")
	if root == "" {
		http.Error(w, "Server configuration error: VC_LOCAL_ROOT not set", http.StatusInternalServerError)
		return
	}

	path := filepath.Join(root, key)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")

	n, err := io.Copy(w, file)

	if n > 0 {
		observability.ProxyTraffic.WithLabelValues("out").Add(float64(n))
	}

	if err != nil {
		fmt.Printf("Error streaming file %s: %v\n", key, err)
	}
}
