package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"psa/internal/config"
	controllerhttp "psa/internal/controller/http"
	"psa/internal/repository/external/hh"
	"psa/internal/repository/external/hh/token"
	"psa/internal/repository/postgresql"
	"psa/internal/repository/redis"
	"psa/internal/usecase/cron"
	"psa/internal/usecase/extractor"
	"psa/internal/usecase/provider"
	"psa/internal/usecase/scraper"
	"psa/pkg/httpserver"
	"psa/pkg/logger/slogx"
	"syscall"
	"time"
)

func Run(cfg *config.Config, log *slog.Logger) error {
	const op = "internal.app.Run"

	db, err := postgresql.New(cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer db.Close()

	cache, err := redis.New(cfg.Redis)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer cache.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	tokenManager := token.NewTokenManager(cfg.HHAuth, log)
	hhClient := hh.New(cfg, log, httpClient, tokenManager)

	skillExtractor := extractor.New()

	scraping := scraper.New(
		log,
		db,
		db,
		db,
		db,
		hhClient,
		skillExtractor,
		cache,
	)

	cronScheduler := cron.New(log, scraping)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	professionProvider := provider.New(log, db, db, db, db, cache)

	// HTTP Router
	handler := controllerhttp.NewRouter(log, professionProvider)

	// HTTP Server
	httpServer := httpserver.New(
		handler,
		httpserver.Port(cfg.HTTPServer.Port),
		httpserver.ReadTimeout(cfg.HTTPServer.Timeout),
		httpserver.WriteTimeout(cfg.HTTPServer.Timeout),
		httpserver.ShutdownTimeout(30*time.Second),
	)

	httpServer.Start()
	log.Info("HTTP server started", "address", cfg.HTTPServer.Host+":"+cfg.HTTPServer.Port)

	// Graceful shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		log.Info(op, slog.String("signal", s.String()))
	case err = <-httpServer.Notify():
		log.Error("HTTP server error", slogx.Err(err))
	}

	if err = httpServer.Shutdown(context.Background()); err != nil {
		log.Error("HTTP server shutdown error", slogx.Err(err))
	}

	log.Info("App stopped gracefully")
	return nil
}
