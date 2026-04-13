package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

const jobTimeout = 1 * time.Hour

type ScrapingProvider interface {
	ProcessActiveProfessionsArchive(ctx context.Context) error
	ProcessActiveProfessionsDaily(ctx context.Context) error
}

type Cron struct {
	log       *slog.Logger
	scraper   ScrapingProvider
	scheduler gocron.Scheduler
}

func New(log *slog.Logger, scraper ScrapingProvider) (*Cron, error) {
	const op = "service.cron.New"
	log = log.With("op", op)

	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Warn("location_load_failed", slogx.Err(err))
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
	const op = "service.cron.Start"
	log := c.log.With("op", op)

	log.Info("scheduler_starting")

	// Monthly job
	_, err := c.scheduler.NewJob(
		gocron.CronJob("0 3 15 * *", false),
		gocron.NewTask(func() {
			c.runScrapingJobArchive(ctx, "monthly")
		}),
		gocron.WithName("monthly_scraping"),
	)
	if err != nil {
		log.Error("monthly_schedule_failed", slogx.Err(err))
		return fmt.Errorf("%s: monthly job: %w", op, err)
	}
	log.Debug("monthly_scheduled", "schedule", "0 3 15 * *")

	// Daily job
	_, err = c.scheduler.NewJob(
		gocron.CronJob("0 3 1-14,16-31 * *", false),
		gocron.NewTask(func() {
			c.runScrapingJobDaily(ctx, "daily")
		}),
		gocron.WithName("daily_scraping"),
	)
	if err != nil {
		log.Error("daily_schedule_failed", slogx.Err(err))
		return fmt.Errorf("%s: daily job: %w", op, err)
	}
	log.Debug("daily_scheduled", "schedule", "0 3 1-14,16-31 * *")

	c.scheduler.Start()
	log.Info("scheduler_started")

	return nil
}

func (c *Cron) Stop(ctx context.Context) error {
	const op = "service.cron.Stop"
	log := c.log.With("op", op)

	log.Info("scheduler_stopping")

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

	case <-ctx.Done():
		log.Error("scheduler_shutdown_timeout", slogx.Err(ctx.Err()))

		select {
		case err := <-done:
			if err != nil {
				log.Warn("scheduler_shutdown_completed_late", slogx.Err(err))
			} else {
				log.Info("scheduler_shutdown_completed_late")
			}
		default:
		}

		return ctx.Err()
	}
}

func (c *Cron) runScrapingJobArchive(ctx context.Context, jobType string) {
	const op = "service.cron.runScrapingJobArchive"
	runID := uuid.New().String()
	log := c.log.With("op", op, "job", jobType, "run_id", runID)

	log.Info("job_started")

	ctxWithLogger := loggerctx.WithLogger(ctx, log)
	ctxJob, cancel := context.WithTimeout(ctxWithLogger, jobTimeout)
	defer cancel()

	if err := c.scraper.ProcessActiveProfessionsArchive(ctxJob); err != nil {
		log.Error("job_failed", slogx.Err(err))
		return
	}

	log.Info("job_completed")
}

func (c *Cron) runScrapingJobDaily(ctx context.Context, jobType string) {
	const op = "service.cron.runScrapingJobDaily"
	runID := uuid.New().String()
	log := c.log.With("op", op, "job", jobType, "run_id", runID)

	log.Info("job_started")

	ctxWithLogger := loggerctx.WithLogger(ctx, log)
	ctxJob, cancel := context.WithTimeout(ctxWithLogger, jobTimeout)
	defer cancel()

	if err := c.scraper.ProcessActiveProfessionsDaily(ctxJob); err != nil {
		log.Error("job_failed", slogx.Err(err))
		return
	}

	log.Info("job_completed")
}
