package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"psa/internal/config"
	controllerhttp "psa/internal/handler/http"
	"psa/internal/handler/http/v1/handler/admin"
	"psa/internal/handler/http/v1/handler/public"
	"psa/internal/integration/hh"
	"psa/internal/repository/postgresql"
	"psa/internal/repository/redis"
	"psa/internal/service/auth"
	"psa/internal/service/cron"
	"psa/internal/service/extractor"
	"psa/internal/service/provider"
	"psa/internal/service/scraper"
	"psa/pkg/httpserver"
	"psa/pkg/jwtmanager"
	"psa/pkg/logger/slogx"
)

func Run(cfg *config.Config, log *slog.Logger) error {
	const op = "app.Run"

	// infrastructure
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

	// external services
	hhClient := hh.NewAdapter(cfg, log)

	// usecases/services
	skillExtractor := extractor.New()

	scraping := scraper.New(
		db,
		db,
		db,
		db,
		hhClient,
		skillExtractor,
		cache,
	)

	cronScheduler, err := cron.New(log, scraping)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	professionProvider := provider.New(db, db, db, db, cache)

	jwtManager := jwtmanager.NewJWT(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
		cfg.JWT.Issuer,
	)

	jwtAdapter := auth.NewJWTAdapter(jwtManager)
	authUC := auth.New(
		db,
		db,
		jwtAdapter,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
	)

	// HTTP handlers v1
	authPublicHandler := public.NewAuthHandler(authUC)
	professionPublicHandler := public.NewProfessionHandler(professionProvider)
	professionAdminHandler := admin.NewProfessionAdminHandler(professionProvider)

	httpHandlers := controllerhttp.V1Handlers{
		AuthPublic:       authPublicHandler,
		ProfessionPublic: professionPublicHandler,
		ProfessionAdmin:  professionAdminHandler,
	}

	// HTTP Router
	router := controllerhttp.NewRouter(
		log,
		httpHandlers,
		authUC,
	)

	// HTTP Server
	httpServer := httpserver.New(
		router,
		httpserver.Port(cfg.HTTPServer.Port),
		httpserver.ReadTimeout(cfg.HTTPServer.Timeout),
		httpserver.WriteTimeout(cfg.HTTPServer.Timeout),
		httpserver.ShutdownTimeout(30*time.Second),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cronScheduler.Start(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := cronScheduler.Stop(stopCtx); err != nil {
			log.Error("cron.stop.failed", slogx.Err(err))
		}
	}()

	httpServer.Start()
	log.Info("http.server.started", "address", cfg.HTTPServer.Host+":"+cfg.HTTPServer.Port)

	var shutdownReason string
	select {
	case <-ctx.Done():
		shutdownReason = "signal"
	case err = <-httpServer.Notify():
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownReason = "http.error"
			log.Error("http.server_failed", slogx.Err(err))
		} else {
			shutdownReason = "http.closed"
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err = httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http.server.shutdown_failed", slogx.Err(err))
	}

	log.Info("app.stopped", "reason", shutdownReason)
	return nil
}
