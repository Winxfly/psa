package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
)

func (s *Storage) SaveArchiveProfessionResult(ctx context.Context, sessionID uuid.UUID,
	scrapedAt time.Time, result domain.ProfessionScrapeResult) error {
	const op = "repository.postgresql.scrape_result.SaveArchiveProfessionResult"

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.Queries.WithTx(tx)

	if err := insertStatDaily(ctx, qtx, result.ProfessionID, result.VacancyCount, scrapedAt); err != nil {
		return fmt.Errorf("%s: save daily stat: %w", op, err)
	}

	if err := insertStat(ctx, qtx, sessionID, result.ProfessionID, result.VacancyCount); err != nil {
		return fmt.Errorf("%s: save stat: %w", op, err)
	}

	if err := insertFormalSkills(ctx, qtx, sessionID, result.ProfessionID, result.FormalSkills); err != nil {
		return fmt.Errorf("%s: save formal skills: %w", op, err)
	}

	if err := insertExtractedSkills(ctx, qtx, sessionID, result.ProfessionID, result.ExtractedSkills); err != nil {
		return fmt.Errorf("%s: save extracted skills: %w", op, err)
	}

	if err := replaceSkillCorpusByProfession(ctx, qtx, result.ProfessionID, result.FormalSkills); err != nil {
		return fmt.Errorf("%s: replace skill corpus: %w", op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit tx: %w", op, err)
	}

	return nil
}

func (s *Storage) SaveDailyProfessionResult(ctx context.Context, scrapedAt time.Time,
	result domain.ProfessionScrapeResult) error {
	const op = "repository.postgresql.scrape_result.SaveDailyProfessionResult"

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.Queries.WithTx(tx)

	if err := insertStatDaily(ctx, qtx, result.ProfessionID, result.VacancyCount, scrapedAt); err != nil {
		return fmt.Errorf("%s: save daily stat: %w", op, err)
	}

	if err := replaceSkillCorpusByProfession(ctx, qtx, result.ProfessionID, result.FormalSkills); err != nil {
		return fmt.Errorf("%s: replace skill corpus: %w", op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit tx: %w", op, err)
	}

	return nil
}
