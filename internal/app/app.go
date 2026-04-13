package app

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

	"psa/internal/app/closer"
	"psa/internal/config"
	controllerhttp "psa/internal/handler/http"
	"psa/internal/handler/http/v1/handler/admin"
	"psa/internal/handler/http/v1/handler/public"
	"psa/internal/health"
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
	closer.Add("db", func(ctx context.Context) error {
		// pgxpool.Close does not accept context; timeout is handled by closer.
		db.Close()
		return nil
	})

	cache, err := redis.New(cfg.Redis)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	closer.Add("cache", func(ctx context.Context) error {
		return cache.Close()
	})

	// external services
	hhClient := hh.NewAdapter(cfg, log)

	// services
	skillExtractor := extractor.New()

	scraping := scraper.New(
		db,
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

	professionProvider := provider.New(db, db, db, db, cache, db)

	// health checks
	healthChecker := health.New(
		health.NewDBCheck(db),
		health.NewCacheCheck(cache),
	)

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
	professionAdminHandler := admin.NewProfessionAdminHandler(professionProvider, scraping)
	trendHandler := public.NewTrendHandler(professionProvider)

	httpHandlers := controllerhttp.V1Handlers{
		AuthPublic:       authPublicHandler,
		ProfessionPublic: professionPublicHandler,
		ProfessionAdmin:  professionAdminHandler,
		Trend:            trendHandler,
	}

	// HTTP Router
	router, err := controllerhttp.NewRouter(
		log,
		httpHandlers,
		authUC,
		cfg.HTTPServer.CORS,
		healthChecker,
	)
	if err != nil {
		return fmt.Errorf("%s: init router: %w", op, err)
	}

	// HTTP Server
	httpServer := httpserver.New(
		router,
		httpserver.Port(cfg.HTTPServer.Port),
		httpserver.ReadTimeout(cfg.HTTPServer.Timeout),
		httpserver.WriteTimeout(cfg.HTTPServer.Timeout),
		httpserver.IdleTimeout(60*time.Second),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cronScheduler.Start(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	closer.Add("cron", func(ctx context.Context) error {
		return cronScheduler.Stop(ctx)
	})

	httpServer.Start()
	log.Info("http_server_started", "addr", cfg.HTTPServer.Host+":"+cfg.HTTPServer.Port)

	var shutdownReason string
	select {
	case <-ctx.Done():
		shutdownReason = "signal"
	case err = <-httpServer.Notify():
		stop() // cancel signal context so resources don't see it
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownReason = "http_error"
			log.Error("http_server_failed", slogx.Err(err))
		} else {
			shutdownReason = "http_closed"
		}
	}

	// Second SIGINT → immediate exit (no graceful shutdown).
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		os.Exit(1)
	}()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err = httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http_server_shutdown_failed", slogx.Err(err))
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeCancel()

	if err = closer.CloseAll(closeCtx, log); err != nil {
		log.Error("resource_close_failed", slogx.Err(err))
	}

	log.Info("app_stopped", "reason", shutdownReason)
	return nil
}
