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

	"github.com/agentzero/server/internal/agent"
	"github.com/agentzero/server/internal/api"
	"github.com/agentzero/server/internal/auth"
	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/llm"
	"github.com/agentzero/server/internal/tools"
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

	verifier := auth.NewAppleVerifier(cfg.AppleBundleID)
	tokens := auth.NewTokenIssuer(cfg.JWTSecret, 30*24*time.Hour)

	// 装备库：在 main 一次性注册所有内置装备。
	registry := tools.NewRegistry()
	registry.Register(&tools.WriteFile{})
	registry.Register(&tools.ReadFile{})
	registry.Register(tools.NewFetchURL())
	registry.Register(tools.NewWebSearch(cfg.BochaAPIKey))

	llmClient := llm.NewClient(cfg.DeepseekAPIKey)
	if cfg.DeepseekBaseURL != "" {
		llmClient.BaseURL = cfg.DeepseekBaseURL
	}

	broker := agent.NewBroker()
	runner := agent.New(
		agent.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			MaxIterations: 16,
		},
		store, llmClient, registry, broker, logger,
	)

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: api.NewRouter(api.Deps{
			DB:       store,
			Apple:    verifier,
			Tokens:   tokens,
			Logger:   logger,
			Runner:   runner,
			Broker:   broker,
			Registry: registry,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		// 注意：WriteTimeout 留 0；SSE 长连接需要写超时不限制。
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", srv.Addr, "db", cfg.DBPath, "workspaces", cfg.WorkspaceRoot)
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
	Port            string
	DBPath          string
	WorkspaceRoot   string
	JWTSecret       string
	AppleBundleID   string
	DeepseekAPIKey  string
	DeepseekBaseURL string
	BochaAPIKey     string
}

func loadConfig() config {
	c := config{
		Port:            envOr("PORT", "8080"),
		DBPath:          envOr("DB_PATH", "/var/lib/agentzero/agentzero.db"),
		WorkspaceRoot:   envOr("WORKSPACE_ROOT", "/var/lib/agentzero/workspaces"),
		JWTSecret:       envOr("JWT_SECRET", ""),
		AppleBundleID:   envOr("APPLE_BUNDLE_ID", "com.agentzero.me"),
		DeepseekAPIKey:  envOr("DEEPSEEK_API_KEY", ""),
		DeepseekBaseURL: envOr("DEEPSEEK_BASE_URL", ""),
		BochaAPIKey:     envOr("BOCHA_API_KEY", ""),
	}
	if c.JWTSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}
	if c.DeepseekAPIKey == "" {
		slog.Error("DEEPSEEK_API_KEY environment variable is required")
		os.Exit(1)
	}
	if err := os.MkdirAll(c.WorkspaceRoot, 0o755); err != nil {
		slog.Error("ensure workspace root failed", "err", err)
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
