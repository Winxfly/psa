package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	postgresql "psa/internal/repository/postgresql/generated"
)

func (s *Storage) SaveStatDaily(ctx context.Context, professionID uuid.UUID, vacancyCount int, scrapedAt time.Time) error {
	const op = "repository.postgresql.stat_daily.SaveStatDaily"

	_, err := s.Queries.InsertStatDaily(ctx, postgresql.InsertStatDailyParams{
		ProfessionID: professionID,
		VacancyCount: int32(vacancyCount),
		ScrapedAt:    scrapedAt,
	})

	return err
}

func (s *Storage) GetStatDailyByProfessionID(ctx context.Context, professionID uuid.UUID) ([]domain.StatDailyPoint, error) {
	const op = "repository.postgresql.stat_daily.GetStatDailyByProfessionID"

	rows, err := s.Queries.GetStatDailyByProfessionID(ctx, professionID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	points := make([]domain.StatDailyPoint, len(rows))
	for i, row := range rows {
		points[i] = domain.StatDailyPoint{
			Date:         row.ScrapedAt,
			VacancyCount: row.VacancyCount,
		}
	}

	return points, nil
}

func (s *Storage) GetStatDailyByProfessionIDs(ctx context.Context, professionIDs []uuid.UUID) (map[uuid.UUID][]domain.StatDailyPoint, error) {
	const op = "repository.postgresql.stat_daily.GetStatDailyByProfessionIDs"

	rows, err := s.Queries.GetStatDailyByProfessionIDs(ctx, professionIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	// Группируем по profession_id
	result := make(map[uuid.UUID][]domain.StatDailyPoint)
	for _, row := range rows {
		result[row.ProfessionID] = append(result[row.ProfessionID], domain.StatDailyPoint{
			Date:         row.ScrapedAt,
			VacancyCount: row.VacancyCount,
		})
	}

	return result, nil
}
