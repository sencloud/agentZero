package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentzero/server/internal/api"
	"github.com/agentzero/server/internal/auth"
	"github.com/agentzero/server/internal/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open db failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := db.Migrate(store); err != nil {
		logger.Error("migrate failed", "err", err)
		os.Exit(1)
	}

	if err := db.SeedIfEmpty(store); err != nil {
		logger.Error("seed failed", "err", err)
		os.Exit(1)
	}

	verifier := auth.NewAppleVerifier(cfg.AppleBundleID)
	tokens := auth.NewTokenIssuer(cfg.JWTSecret, 30*24*time.Hour)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewRouter(store, verifier, tokens, logger),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", srv.Addr, "db", cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown initiated")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "err", err)
	}
}

type config struct {
	Port          string
	DBPath        string
	JWTSecret     string
	AppleBundleID string
}

func loadConfig() config {
	c := config{
		Port:          envOr("PORT", "8080"),
		DBPath:        envOr("DB_PATH", "/var/lib/agentzero/agentzero.db"),
		JWTSecret:     envOr("JWT_SECRET", ""),
		AppleBundleID: envOr("APPLE_BUNDLE_ID", "com.agentzero.me"),
	}
	if c.JWTSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}
	return c
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
