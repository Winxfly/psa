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
	"psa/internal/usecase"
	"psa/internal/usecase/provider"
	"psa/internal/usecase/scheduler"
	"psa/internal/usecase/scraper"
	"psa/pkg/httpserver"
	"psa/pkg/logger/slogx"
	"syscall"
	"time"
)

func Run(cfg *config.Config, log *slog.Logger) error {
	const op = "internal.app.Run"

	repo, err := postgresql.New(cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer repo.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	tokenManager := token.NewTokenManager(cfg.HHAuth, log)
	hhClient := hh.New(log, httpClient, cfg, tokenManager)

	scraping := scraper.New(hhClient, repo, log)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := scraping.ProcessActiveProfessions(ctx); err != nil {
		log.Error("Failed immediate scraping", slogx.Err(err))
	} else {
		log.Info("Immediate scraping completed successfully")
	}

	schedule := scheduler.NewScheduler(scraping, repo, log)
	schedule.StartMonthlySchedule()
	defer schedule.Stop()

	professionProvider := provider.New(repo)

	// HTTP Router
	handler := controllerhttp.NewRouter(log, professionProvider)

	// HTTP Server
	httpServer := httpserver.New(
		handler,
		httpserver.Port(cfg.HTTPServer.Port),
		httpserver.ReadTimeout(cfg.HTTPServer.Timeout),
		httpserver.WriteTimeout(cfg.HTTPServer.Timeout),
		httpserver.ShutdownTimeout(10*time.Second),
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
		if err != nil {
			log.Error(op, slogx.Err(err))
		}
	}

	if err = httpServer.Shutdown(context.Background()); err != nil {
		log.Error(op, slogx.Err(err))
	}

	log.Info("App stopped gracefully")
	return nil
}

func startScrapingWithDelay(cfg *config.Config, log *slog.Logger, repo usecase.Repository, delay time.Duration) {
	log.Info("Scraping will start after delay", "delay", delay)

	time.Sleep(delay)

	log.Info("Starting scraping process")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	tokenManager := token.NewTokenManager(cfg.HHAuth, log)
	hhClient := hh.New(log, httpClient, cfg, tokenManager)

	scraping := scraper.New(
		hhClient,
		repo,
		log,
	)

	if err := scraping.ProcessActiveProfessions(ctx); err != nil {
		log.Error("Scraping failed", slogx.Err(err))
	} else {
		log.Info("Scraping completed successfully")
	}
}
