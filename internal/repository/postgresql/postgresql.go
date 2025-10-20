package postgresql

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/config"
	"psa/internal/entity"
	postgresql "psa/internal/repository/postgresql/generated"
	"psa/internal/usecase"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ usecase.Repository = (*Storage)(nil)

type Storage struct {
	Pool    *pgxpool.Pool
	Queries *postgresql.Queries
}

func New(cfg config.StoragePath) (*Storage, error) {
	const op = "repository.postgresql.New"

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Username, cfg.Password, cfg.Host,
		cfg.Port, cfg.Database, cfg.SSLMode)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w:", op, err)
	}

	err = pool.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("%s: %w:", op, err)
	}

	queries := postgresql.New(pool)

	return &Storage{Pool: pool,
		Queries: queries,
	}, nil
}

func (s *Storage) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}

// GetActiveProfessions return active profession
func (s *Storage) GetActiveProfessions(ctx context.Context) ([]entity.Profession, error) {
	const op = "repository.postgresql.GetActiveProfessions"

	rows, err := s.Queries.GetActiveProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	professions := make([]entity.Profession, len(rows))
	for i, row := range rows {
		professions[i] = entity.Profession{
			ID:           row.ID,
			Name:         row.Name,
			VacancyQuery: row.VacancyQuery,
			IsActive:     true,
		}
	}

	return professions, nil
}

func (s *Storage) GetAllProfessions(ctx context.Context) ([]entity.Profession, error) {
	const op = "repository.postgresql.GetAllProfessions"

	rows, err := s.Queries.GetAllProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	professions := make([]entity.Profession, len(rows))
	for i, row := range rows {
		professions[i] = entity.Profession{
			ID:           row.ID,
			Name:         row.Name,
			VacancyQuery: row.VacancyQuery,
			IsActive:     row.IsActive,
		}
	}

	return professions, nil
}

func (s *Storage) GetProfessionByID(ctx context.Context, id uuid.UUID) (entity.Profession, error) {
	const op = "repository.postgresql.GetProfessionByID"

	row, err := s.Queries.GetProfessionByID(ctx, id)
	if err != nil {
		return entity.Profession{}, fmt.Errorf("%s: %w", op, err)
	}

	return entity.Profession{
		ID:           row.ID,
		Name:         row.Name,
		VacancyQuery: row.VacancyQuery,
		IsActive:     row.IsActive,
	}, nil
}

func (s *Storage) GetProfessionByName(ctx context.Context, name string) (entity.Profession, error) {
	const op = "repository.postgresql.GetProfessionByName"

	row, err := s.Queries.GetProfessionByName(ctx, name)
	if err != nil {
		return entity.Profession{}, fmt.Errorf("%s: %w", op, err)
	}

	return entity.Profession{
		ID:           row.ID,
		Name:         row.Name,
		VacancyQuery: row.VacancyQuery,
		IsActive:     row.IsActive,
	}, nil
}

func (s *Storage) AddProfession(ctx context.Context, profession entity.Profession) (uuid.UUID, error) {
	const op = "repository.postgresql.AddProfession"

	return s.Queries.InsertProfession(ctx, postgresql.InsertProfessionParams{
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})
}

func (s *Storage) UpdateProfession(ctx context.Context, profession entity.Profession) error {
	const op = "repository.postgresql.UpdateProfession"

	return s.Queries.UpdateProfession(ctx, postgresql.UpdateProfessionParams{
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})
}

func (s *Storage) CreateScrapingSession(ctx context.Context) (uuid.UUID, error) {
	return s.Queries.InsertScrapingDate(ctx)
}

func (s *Storage) GetLatestScraping(ctx context.Context) (entity.Scraping, error) {
	const op = "repository.postgresql.GetLatestScraping"

	row, err := s.Queries.GetLatestScraping(ctx)
	if err != nil {
		return entity.Scraping{}, fmt.Errorf("%s: %w", op, err)
	}

	return entity.Scraping{
		ID:        row.ID,
		ScrapedAt: row.ScrapedAt,
	}, nil
}

func (s *Storage) GetAllScrapingDates(ctx context.Context) ([]entity.Scraping, error) {
	const op = "repository.postgresql.GetAllScrapingDates"

	rows, err := s.Queries.GetAllScrapingDates(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	scrapings := make([]entity.Scraping, len(rows))
	for i, row := range rows {
		scrapings[i] = entity.Scraping{
			ID:        row.ID,
			ScrapedAt: row.ScrapedAt,
		}
	}

	return scrapings, nil
}

func (s *Storage) ExistsScrapingSessionInCurrMonth(ctx context.Context) (bool, error) {
	const op = "repository.postgresql.ExistsScrapingSessionInCurrMonth"

	const query = "SELECT EXISTS (SELECT 1 FROM scraping WHERE date_trunc('month', scraped_at) = date_trunc('month', CURRENT_DATE))"

	var exists bool
	err := s.Pool.QueryRow(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func (s *Storage) SaveFormalSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.SaveFormalSkills"

	params := make([]postgresql.InsertFormalSkillsParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, postgresql.InsertFormalSkillsParams{
			ProfessionID: professionID,
			Skill:        skill,
			Count:        int32(count),
			ScrapedAtID:  sessionID,
		})
	}

	_, err := s.Queries.InsertFormalSkills(ctx, params)

	return err
}

func (s *Storage) SaveExtractedSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.SaveExtractedSkills"

	params := make([]postgresql.InsertExtractedSkillsParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, postgresql.InsertExtractedSkillsParams{
			ProfessionID: professionID,
			Skill:        skill,
			Count:        int32(count),
			ScrapedAtID:  sessionID,
		})
	}

	_, err := s.Queries.InsertExtractedSkills(ctx, params)

	return err
}

func (s *Storage) GetFormalSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error) {
	const op = "repository.postgresql.GetFormalSkillsByProfessionAndDate"

	rows, err := s.Queries.GetFormalSkillsByProfessionAndDate(ctx, postgresql.GetFormalSkillsByProfessionAndDateParams{
		ProfessionID: professionID,
		ScrapedAtID:  scrapedAtID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	skills := make([]entity.Skill, len(rows))
	for i, row := range rows {
		skills[i] = entity.Skill{
			Skill: row.Skill,
			Count: row.Count,
		}
	}

	return skills, nil
}

func (s *Storage) GetExtractedSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error) {
	const op = "repository.postgresql.GetExtractedSkillsByProfessionAndDate"

	rows, err := s.Queries.GetExtractedSkillsByProfessionAndDate(ctx, postgresql.GetExtractedSkillsByProfessionAndDateParams{
		ProfessionID: professionID,
		ScrapedAtID:  scrapedAtID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	skills := make([]entity.Skill, len(rows))
	for i, row := range rows {
		skills[i] = entity.Skill{
			Skill: row.Skill,
			Count: row.Count,
		}
	}

	return skills, nil
}

func (s *Storage) SaveStat(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, vacancyCount int) error {
	const op = "repository.postgresql.SaveStat"

	_, err := s.Queries.InsertStat(ctx, postgresql.InsertStatParams{
		ProfessionID: professionID,
		VacancyCount: int32(vacancyCount),
		ScrapedAtID:  sessionID,
	})

	return err
}

func (s *Storage) GetLatestStatByProfessionID(ctx context.Context, professionID uuid.UUID) (entity.Stat, error) {
	const op = "repository.postgresql.GetLatestStatByProfessionID"

	row, err := s.Queries.GetLatestStatByProfessionID(ctx, professionID)
	if err != nil {
		return entity.Stat{}, fmt.Errorf("%s: %w", op, err)
	}

	return entity.Stat{
		ProfessionID: row.ProfessionID,
		VacancyCount: row.VacancyCount,
		ScrapedAtID:  row.ScrapedAtID,
	}, nil
}

func (s *Storage) GetStatsByProfessionsAndDateRange(ctx context.Context, professionIDs []uuid.UUID, startDate, endDate string) ([]entity.Stat, error) {
	const op = "repository.postgresql.GetStatsByProfessionAndDateRange"

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

	stats := make([]entity.Stat, len(rows))
	for i, row := range rows {
		stats[i] = entity.Stat{
			ProfessionID: row.ProfessionID,
			VacancyCount: row.VacancyCount,
			ScrapedAtID:  row.ScrapedAtID,
		}
	}

	return stats, nil
}
