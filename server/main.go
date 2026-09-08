package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"intrachat/server/internal/app"
	"intrachat/server/internal/config"
	"intrachat/server/internal/store"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DatabaseURL, migrations)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.EnsureInitialAdmin(ctx, cfg.InitialAdminUsername, cfg.InitialAdminPassword); err != nil {
		slog.Error("initial administrator failed", "error", err)
		os.Exit(1)
	}
	var minio *store.Minio
	if m, err := store.NewMinio(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecret, cfg.MinIOBucket, cfg.MinIOUseSSL); err != nil {
		slog.Warn("minio client unavailable, file upload disabled", "error", err)
	} else if err := m.EnsureBucket(ctx); err != nil {
		slog.Warn("minio bucket unavailable, file upload disabled", "error", err)
	} else {
		minio = m
		slog.Info("minio ready", "endpoint", cfg.MinIOEndpoint, "bucket", cfg.MinIOBucket)
	}
	handler := app.New(cfg, db, minio)
	server := &http.Server{Addr: cfg.ListenAddress, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("chat server listening", "address", cfg.ListenAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
