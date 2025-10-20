package scheduler

import (
	"context"
	"log/slog"
	"psa/internal/usecase"
	"time"
)

type Scheduler struct {
	scraper usecase.Scraper
	repo    usecase.Repository
	log     *slog.Logger
	ticker  *time.Ticker
	done    chan bool
}

func NewScheduler(scraper usecase.Scraper, repo usecase.Repository, log *slog.Logger) *Scheduler {
	return &Scheduler{
		scraper: scraper,
		repo:    repo,
		log:     log,
		done:    make(chan bool),
	}
}

func (s *Scheduler) StartMonthlySchedule() {
	s.ticker = time.NewTicker(24 * time.Hour)

	go func() {
		for {
			select {
			case <-s.ticker.C:
				now := time.Now()
				if now.Day() == 15 && now.Hour() == 3 {
					s.tryRunScraping()
				}
			case <-s.done:
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}

	s.done <- true
}

func (s *Scheduler) tryRunScraping() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Minute)
	defer cancel()

	exists, err := s.repo.ExistsScrapingSessionInCurrMonth(ctx)
	if err != nil {
		s.log.Error("Failed to check scraping session existence", "error", err)
		return
	}

	if exists {
		s.log.Info("Scraping session already exists for current month, skipping")
		return
	}

	s.log.Info("Starting monthly scraping session")

	if err := s.scraper.ProcessActiveProfessions(ctx); err != nil {
		s.log.Error("Failed monthly scraping session", "error", err)
	} else {
		s.log.Info("Monthly scraping completed successfully")
	}
}
