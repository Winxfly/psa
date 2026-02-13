package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

const jobTimeout = 3 * time.Hour

type ScrapingProvider interface {
	ProcessActiveProfessions(ctx context.Context, saveToDB bool) error
}

type Cron struct {
	log       *slog.Logger
	scraper   ScrapingProvider
	scheduler gocron.Scheduler
}

func New(log *slog.Logger, scraper ScrapingProvider) (*Cron, error) {
	const op = "usecase.cron.New"
	log = log.With("op", op)

	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Warn("location.load_failed", slogx.Err(err))
		location = time.UTC
	}

	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(location),
		gocron.WithGlobalJobOptions(
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		),
	)
	if err != nil {
		log.Error("init_failed", slogx.Err(err))
		return nil, fmt.Errorf("%s: init scheduler: %w", op, err)
	}

	return &Cron{
		log:       log,
		scraper:   scraper,
		scheduler: scheduler,
	}, nil
}

func (c *Cron) Start(ctx context.Context) error {
	const op = "usecase.cron.Start"
	log := c.log.With("op", op)

	log.Info("scheduler_starting")

	// Monthly job
	_, err := c.scheduler.NewJob(
		gocron.CronJob("0 3 15 * *", false),
		gocron.NewTask(func() {
			c.runScrapingJob(ctx, true, "monthly")
		}),
		gocron.WithName("monthly_scraping"),
	)
	if err != nil {
		log.Error("monthly.schedule_failed", slogx.Err(err))
		return fmt.Errorf("%s: monthly job: %w", op, err)
	}
	log.Debug("monthly.scheduled", "schedule", "0 3 15 * *")

	// Daily job
	_, err = c.scheduler.NewJob(
		gocron.CronJob("0 3 1-14,16-31 * *", false),
		gocron.NewTask(func() {
			c.runScrapingJob(ctx, false, "daily")
		}),
		gocron.WithName("daily_scraping"),
	)
	if err != nil {
		log.Error("daily.schedule_failed", slogx.Err(err))
		return fmt.Errorf("%s: daily job: %w", op, err)
	}
	log.Debug("daily.scheduled", "schedule", "0 3 1-14,16-31 * *")

	c.scheduler.Start()
	log.Info("scheduler_started")

	return nil
}

func (c *Cron) Stop(ctx context.Context) error {
	const op = "usecase.cron.Stop"
	log := c.log.With("op", op)

	log.Info("scheduler_stopping")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- c.scheduler.Shutdown()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Error("scheduler_shutdown_failed", slogx.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
		log.Info("scheduler_stopped")
		return nil
	case <-shutdownCtx.Done():
		log.Error("scheduler_shutdown_timeout")
		return shutdownCtx.Err()
	}
}

func (c *Cron) runScrapingJob(ctx context.Context, saveToDB bool, jobType string) {
	const op = "usecase.cron.runScrapingJob"
	log := c.log.With("op", op, "job", jobType)

	start := time.Now()
	defer func() {
		log.Debug("finished", "duration", time.Since(start))
	}()

	log.Info("started")

	ctxWithLogger := loggerctx.WithLogger(ctx, log)
	ctxJob, cancel := context.WithTimeout(ctxWithLogger, jobTimeout)
	defer cancel()

	if err := c.scraper.ProcessActiveProfessions(ctxJob, saveToDB); err != nil {
		log.Error("failed", slogx.Err(err))
		return
	}

	log.Info("completed")
}
