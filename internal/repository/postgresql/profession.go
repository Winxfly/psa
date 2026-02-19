package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"psa/internal/domain"
	postgresql "psa/internal/repository/postgresql/generated"
)

const (
	pgErrUniqueViolation = "23505"
)

// GetActiveProfessions return active profession
func (s *Storage) GetActiveProfessions(ctx context.Context) ([]domain.Profession, error) {
	const op = "repository.postgresql.profession.GetActiveProfessions"

	rows, err := s.Queries.GetActiveProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	professions := make([]domain.Profession, len(rows))
	for i, row := range rows {
		professions[i] = domain.Profession{
			ID:           row.ID,
			Name:         row.Name,
			VacancyQuery: row.VacancyQuery,
			IsActive:     true,
		}
	}

	return professions, nil
}

func (s *Storage) GetAllProfessions(ctx context.Context) ([]domain.Profession, error) {
	const op = "repository.postgresql.profession.GetAllProfessions"

	rows, err := s.Queries.GetAllProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	professions := make([]domain.Profession, len(rows))
	for i, row := range rows {
		professions[i] = domain.Profession{
			ID:           row.ID,
			Name:         row.Name,
			VacancyQuery: row.VacancyQuery,
			IsActive:     row.IsActive,
		}
	}

	return professions, nil
}

func (s *Storage) GetProfessionByID(ctx context.Context, id uuid.UUID) (domain.Profession, error) {
	const op = "repository.postgresql.profession.GetProfessionByID"

	row, err := s.Queries.GetProfessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Profession{}, domain.ErrProfessionNotFound
		}
		return domain.Profession{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Profession{
		ID:           row.ID,
		Name:         row.Name,
		VacancyQuery: row.VacancyQuery,
		IsActive:     row.IsActive,
	}, nil
}

func (s *Storage) GetProfessionByName(ctx context.Context, name string) (domain.Profession, error) {
	const op = "repository.postgresql.profession.GetProfessionByName"

	row, err := s.Queries.GetProfessionByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Profession{}, domain.ErrProfessionNotFound
		}
		return domain.Profession{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Profession{
		ID:           row.ID,
		Name:         row.Name,
		VacancyQuery: row.VacancyQuery,
		IsActive:     row.IsActive,
	}, nil
}

func (s *Storage) AddProfession(ctx context.Context, profession domain.Profession) (uuid.UUID, error) {
	const op = "repository.postgresql.profession.AddProfession"

	id, err := s.Queries.InsertProfession(ctx, postgresql.InsertProfessionParams{
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrUniqueViolation {
			return uuid.Nil, domain.ErrProfessionAlreadyExists
		}
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) UpdateProfession(ctx context.Context, profession domain.Profession) error {
	const op = "repository.postgresql.profession.UpdateProfession"

	ct, err := s.Pool.Exec(ctx,
		`UPDATE profession SET name = $2, vacancy_query = $3, is_active = $4 WHERE id = $1`,
		profession.ID, profession.Name, profession.VacancyQuery, profession.IsActive,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrUniqueViolation {
			return domain.ErrProfessionAlreadyExists
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	if ct.RowsAffected() == 0 {
		return domain.ErrProfessionNotFound
	}

	return nil
}
