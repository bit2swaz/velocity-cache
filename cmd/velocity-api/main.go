package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bit2swaz/velocity-cache/internal/api"
	"github.com/bit2swaz/velocity-cache/internal/api/ratelimit"
	"github.com/bit2swaz/velocity-cache/internal/database"
	"github.com/bit2swaz/velocity-cache/internal/storage"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	bucket := os.Getenv("VELOCITY_BUCKET")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s3Client, err := storage.NewS3Client(ctx, bucket)
	if err != nil {
		log.Fatalf("failed to create s3 client: %v", err)
	}

	dbPool, err := database.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	uploadLimit := parseEnvInt("VELOCITY_UPLOAD_LIMIT_PER_HOUR", 100)
	var limiter *ratelimit.Limiter
	if uploadLimit > 0 {
		limiter = ratelimit.New(uploadLimit, time.Hour)
	}

	presignExpiry := 5 * time.Minute
	if v := os.Getenv("VELOCITY_PRESIGN_EXPIRY_SECONDS"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			presignExpiry = time.Duration(seconds) * time.Second
		}
	}

	apiServer := api.NewServer(dbPool, s3Client, limiter, presignExpiry)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("INFO: velocity-api listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server exited with error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("ERROR: graceful shutdown failed: %v", err)
	}

	log.Println("INFO: velocity-api stopped")
}

func parseEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("WARN: invalid integer for %s=%q, using fallback %d", key, raw, fallback)
		return fallback
	}

	if v < 0 {
		return 0
	}

	return v
}
