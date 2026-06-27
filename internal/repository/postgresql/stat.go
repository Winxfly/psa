package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	generated "psa/internal/repository/postgresql/generated"
)

func (s *Storage) SaveStat(ctx context.Context, sessionID uuid.UUID,
	professionID uuid.UUID, vacancyCount int) error {

	return insertStat(ctx, s.Queries, sessionID, professionID, vacancyCount)
}

func insertStat(ctx context.Context, q *generated.Queries, sessionID uuid.UUID,
	professionID uuid.UUID, vacancyCount int) error {
	const op = "repository.postgresql.stat.insertStat"

	_, err := q.InsertStat(ctx, generated.InsertStatParams{
		ProfessionID: professionID,
		VacancyCount: int32(vacancyCount),
		ScrapedAtID:  sessionID,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
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

func (s *Storage) GetStatsByProfessionsAndDateRange(ctx context.Context, professionIDs []uuid.UUID,
	startDate, endDate string) ([]domain.Stat, error) {
	const op = "repository.postgresql.stat.GetStatsByProfessionAndDateRange"

	start, err := time.Parse(time.RFC3339, startDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	end, err := time.Parse(time.RFC3339, endDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	rows, err := s.Queries.GetStatsByProfessionsAndDateRange(ctx, generated.GetStatsByProfessionsAndDateRangeParams{
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
