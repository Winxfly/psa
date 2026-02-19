package postgresql

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/domain"
	postgresql "psa/internal/repository/postgresql/generated"
	"time"
)

func (s *Storage) SaveStat(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, vacancyCount int) error {
	const op = "repository.postgresql.stat.SaveStat"

	_, err := s.Queries.InsertStat(ctx, postgresql.InsertStatParams{
		ProfessionID: professionID,
		VacancyCount: int32(vacancyCount),
		ScrapedAtID:  sessionID,
	})

	return err
}

func (s *Storage) GetLatestStatByProfessionID(ctx context.Context, professionID uuid.UUID) (domain.Stat, error) {
	const op = "repository.postgresql.stat.GetLatestStatByProfessionID"

	row, err := s.Queries.GetLatestStatByProfessionID(ctx, professionID)
	if err != nil {
		return domain.Stat{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Stat{
		ProfessionID: row.ProfessionID,
		VacancyCount: row.VacancyCount,
		ScrapedAtID:  row.ScrapedAtID,
	}, nil
}

func (s *Storage) GetStatsByProfessionsAndDateRange(ctx context.Context, professionIDs []uuid.UUID, startDate, endDate string) ([]domain.Stat, error) {
	const op = "repository.postgresql.stat.GetStatsByProfessionAndDateRange"

	start, err := time.Parse(time.RFC3339, startDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	end, err := time.Parse(time.RFC3339, endDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	rows, err := s.Queries.GetStatsByProfessionsAndDateRange(ctx, postgresql.GetStatsByProfessionsAndDateRangeParams{
		Column1:     professionIDs,
		ScrapedAt:   start,
		ScrapedAt_2: end,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stats := make([]domain.Stat, len(rows))
	for i, row := range rows {
		stats[i] = domain.Stat{
			ProfessionID: row.ProfessionID,
			VacancyCount: row.VacancyCount,
			ScrapedAtID:  row.ScrapedAtID,
		}
	}

	return stats, nil
}
