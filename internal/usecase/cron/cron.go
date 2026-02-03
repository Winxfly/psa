package cron

import (
	"context"
	"github.com/go-co-op/gocron/v2"
	"log/slog"
	"time"
)

type ScrapingProvider interface {
	ProcessActiveProfessions(ctx context.Context, saveToDB bool) error
}

type Cron struct {
	log       *slog.Logger
	scraper   ScrapingProvider
	scheduler gocron.Scheduler
}

func New(log *slog.Logger, scraper ScrapingProvider) *Cron {
	moscowLoc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Error("Failed to load location of Moscow in cron", "error", err)
		moscowLoc = time.UTC
	}

	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(moscowLoc),
		gocron.WithGlobalJobOptions(
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		),
	)
	if err != nil {
		log.Error("failed to initialize gocron", "error", err)
	}

	return &Cron{
		log:       log,
		scraper:   scraper,
		scheduler: scheduler,
	}
}

func (c *Cron) Start() {
	c.log.Info("Starting cron scheduler")

	_, err := c.scheduler.NewJob(
		gocron.CronJob("0 3 15 * *", false),
		gocron.NewTask(c.monthlyScrapingJobs),
		gocron.WithName("monthly_scraping"),
	)
	if err != nil {
		c.log.Error("Failed to start monthly scraping job", "error", err)
		return
	}

	_, err = c.scheduler.NewJob(
		gocron.CronJob("0 3 1-14,16-31 * *", false),
		gocron.NewTask(c.dailyScrapingJobs),
		gocron.WithName("daily_scraping"),
	)
	if err != nil {
		c.log.Error("Failed to start daily scraping job", "error", err)
		return
	}

	c.scheduler.Start()
	c.log.Info("Cron scheduler started")
}

func (c *Cron) Stop() {
	c.log.Info("Stopping cron scheduler")

	if c.scheduler != nil {
		err := c.scheduler.Shutdown()
		if err != nil {
			c.log.Error("Error shutting down cron scheduler", "error", err)
		} else {
			c.log.Info("Cron scheduler stopped")
		}
	}
}

func (c *Cron) monthlyScrapingJobs() {
	c.log.Info("Starting monthly scraping jobs")

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Minute)
	defer cancel()

	if err := c.scraper.ProcessActiveProfessions(ctx, true); err != nil {
		c.log.Error("Monthly scraping job failed", "error", err)
	} else {
		c.log.Info("Monthly scraping job completed")
	}
}

func (c *Cron) dailyScrapingJobs() {
	c.log.Info("Starting daily scraping jobs")

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Minute)
	defer cancel()

	if err := c.scraper.ProcessActiveProfessions(ctx, false); err != nil {
		c.log.Error("Daily scraping job failed", "error", err)
	} else {
		c.log.Info("Daily scraping job completed")
	}
}
