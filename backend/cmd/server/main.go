// Command server is the HTTP entry point. It loads config, builds the wired
// application (app.Build is the single composition root), mounts the HTTP
// delivery, and runs with graceful shutdown.
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

	"github.com/joho/godotenv"

	httpadapter "github.com/vngcloud/agentt/internal/adapter/http"
	"github.com/vngcloud/agentt/internal/app"
	"github.com/vngcloud/agentt/internal/config"
	"github.com/vngcloud/agentt/internal/scheduler"
)

func main() {
	_ = godotenv.Load() // load .env if present; no-op in production when file is absent

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	application, err := app.Build(cfg, logger)
	if err != nil {
		logger.Error("build application failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = application.Close() }()

	router := httpadapter.NewRouter(httpadapter.RouterDeps{
		Chat:           application.Chat,
		Digest:         application.Digest,
		AllowedOrigins: cfg.AllowedOrigins,
		StaticDir:      cfg.StaticDir,
	})

	// Daily scheduler: fires the digest job once per day at a fixed time. Its
	// context is cancelled on shutdown so the goroutine exits cleanly.
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	scheduler.NewDaily(func(ctx context.Context, date string) error {
		_, err := application.Digest.GenerateDaily(ctx, date)
		return err
	}, logger).Start(schedCtx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server until a termination signal arrives.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("server failed", "error", err)
		os.Exit(1)
	case sig := <-stop:
		logger.Info("shutting down", "signal", sig.String())
	}
	schedCancel() // stop the daily scheduler

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
