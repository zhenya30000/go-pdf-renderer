package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhenya3000/go-pdf-renderer/internal/config"
	"github.com/zhenya3000/go-pdf-renderer/internal/httpapi"
	"github.com/zhenya3000/go-pdf-renderer/internal/pdf"
)

var version = "dev"
var commit = "none"
var buildDate = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	renderer, err := pdf.NewChromeRenderer(context.Background(), pdf.ChromeConfig{
		ExecutablePath:    cfg.ChromePath,
		NoSandbox:         cfg.ChromeNoSandbox,
		AllowJavaScript:   cfg.AllowJavaScript,
		AllowedAssetHosts: cfg.AllowedAssetHosts,
	})
	if err != nil {
		return fmt.Errorf("start chrome: %w", err)
	}
	defer renderer.Close()

	handler := httpapi.New(httpapi.Config{
		Renderer:        renderer,
		Logger:          logger,
		MaxConcurrent:   cfg.MaxConcurrent,
		QueueTimeout:    cfg.QueueTimeout,
		RenderTimeout:   cfg.RenderTimeout,
		ReadyTimeout:    cfg.ReadyTimeout,
		MaxRequestBytes: cfg.MaxRequestBytes,
		MaxPDFBytes:     cfg.MaxPDFBytes,
		APIKey:          cfg.APIKey,
		AllowedOrigins:  cfg.AllowedOrigins,
		Version:         version,
		Commit:          commit,
		BuildDate:       buildDate,
	})

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("service started", "address", cfg.Address, "version", version)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-signalCtx.Done():
	case err = <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	select {
	case err = <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
	case <-time.After(time.Second):
	}

	logger.Info("service stopped")
	return nil
}
