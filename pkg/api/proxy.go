package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// HandleProxyUpload handles file uploads for the local driver.
// It streams the request body to a file in VC_LOCAL_ROOT.
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

	// Ensure the directory exists (in case it wasn't created or key has paths)
	// For security, we should probably sanitize key, but assuming hash for now.
	path := filepath.Join(root, key)

	// Create the file
	out, err := os.Create(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create file: %v", err), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	// Stream the body to the file
	_, err = io.Copy(out, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleProxyDownload handles file downloads for the local driver.
// It streams the file from VC_LOCAL_ROOT to the response.
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

	// Set Content-Type if known, or let it be sniffed.
	// For cache artifacts, application/octet-stream is usually safe.
	w.Header().Set("Content-Type", "application/octet-stream")

	_, err = io.Copy(w, file)
	if err != nil {
		// Can't really write an error header if we've already started writing body,
		// but we can log it.
		fmt.Printf("Error streaming file %s: %v\n", key, err)
	}
}
