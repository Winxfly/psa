package postgresql

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"psa/internal/domain"
)

func (s *Storage) CreateScrapingSession(ctx context.Context) (uuid.UUID, error) {
	return s.Queries.InsertScrapingDate(ctx)
}

func (s *Storage) GetLatestScraping(ctx context.Context) (domain.Scraping, error) {
	const op = "repository.postgresql.scraping.GetLatestScraping"

	row, err := s.Queries.GetLatestScraping(ctx)
	if err != nil {
		return domain.Scraping{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Scraping{
		ID:        row.ID,
		ScrapedAt: row.ScrapedAt,
	}, nil
}

func (s *Storage) GetAllScrapingDates(ctx context.Context) ([]domain.Scraping, error) {
	const op = "repository.postgresql.scraping.GetAllScrapingDates"

	rows, err := s.Queries.GetAllScrapingDates(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	scrapings := make([]domain.Scraping, len(rows))
	for i, row := range rows {
		scrapings[i] = domain.Scraping{
			ID:        row.ID,
			ScrapedAt: row.ScrapedAt,
		}
	}

	return scrapings, nil
}

func (s *Storage) ExistsScrapingSessionInCurrMonth(ctx context.Context) (bool, error) {
	const op = "repository.postgresql.scraping.ExistsScrapingSessionInCurrMonth"

	const query = "SELECT EXISTS (SELECT 1 FROM scraping WHERE date_trunc('month', scraped_at) = date_trunc('month', CURRENT_DATE))"

	var exists bool
	err := s.Pool.QueryRow(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}
