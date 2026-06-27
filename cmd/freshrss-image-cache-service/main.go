package main

import (
	"log"

	"github.com/stek29/freshrss-image-cache-service/internal/app"
	"github.com/stek29/freshrss-image-cache-service/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	logger := app.Logger(cfg.Logging)
	server := app.BuildServer(cfg, logger)
	logger.Info("starting server", "listen", cfg.Listen, "data_dir", cfg.DataDir)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
