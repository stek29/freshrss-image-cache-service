package app

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/stek29/freshrss-image-cache-service/internal/config"
	"github.com/stek29/freshrss-image-cache-service/internal/fetch"
	"github.com/stek29/freshrss-image-cache-service/internal/storage"
)

func BuildServer(cfg config.Config, logger *slog.Logger) *http.Server {
	store := storage.NewFileSystemStore(cfg.DataDir)
	fetcher := fetch.NewClient(cfg.HTTPClient)
	service := NewService(cfg, store, fetcher, logger)
	handler := NewHandler(service, cfg.AccessToken, cfg.CORS, logger)
	return &http.Server{
		Addr:    cfg.Listen,
		Handler: handler.Routes(),
	}
}

func Logger(cfg config.Logging) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
