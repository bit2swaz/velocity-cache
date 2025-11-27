package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/bit2swaz/velocity-cache/pkg/api"
	"github.com/bit2swaz/velocity-cache/pkg/storage"
	"github.com/bit2swaz/velocity-cache/pkg/storage/local"
	"github.com/bit2swaz/velocity-cache/pkg/storage/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	port := os.Getenv("VC_PORT")
	if port == "" {
		port = "8080"
	}

	authToken := os.Getenv("VC_AUTH_TOKEN")
	driverType := os.Getenv("VC_STORAGE_DRIVER")
	if driverType == "" {
		driverType = "local"
	}

	var store storage.Driver
	var err error
	ctx := context.Background()

	switch driverType {
	case "s3":
		store, err = s3.New(ctx)
	case "local":
		store, err = local.New()
	default:
		log.Fatalf("Unknown driver: %s", driverType)
	}

	if err != nil {
		log.Fatalf("Failed to initialize storage driver: %v", err)
	}

	handler := api.NewHandler(store)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up"}`))
	})

	r.Group(func(r chi.Router) {

		if authToken != "" {
			r.Use(AuthMiddleware(authToken))
		} else {
			log.Println("WARNING: Running without VC_AUTH_TOKEN. API is public.")
		}

		r.Post("/v1/negotiate", handler.HandleNegotiate)

		if driverType == "local" {
			r.Put("/v1/proxy/blob/{key}", handler.HandleProxyUpload)
			r.Get("/v1/proxy/blob/{key}", handler.HandleProxyDownload)
		}
	})

	log.Printf("Velocity Server v3.0 starting on :%s using driver '%s'", port, driverType)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != token {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
