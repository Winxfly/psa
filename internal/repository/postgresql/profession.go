package postgresql

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"psa/internal/entity"
	postgresql "psa/internal/repository/postgresql/generated"
)

const (
	pgErrUniqueViolation = "23505"
)

// GetActiveProfessions return active profession
func (s *Storage) GetActiveProfessions(ctx context.Context) ([]entity.Profession, error) {
	const op = "repository.postgresql.profession.GetActiveProfessions"

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
	const op = "repository.postgresql.profession.GetAllProfessions"

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
	const op = "repository.postgresql.profession.GetProfessionByID"

	row, err := s.Queries.GetProfessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Profession{}, entity.ErrProfessionNotFound
		}
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
	const op = "repository.postgresql.profession.GetProfessionByName"

	row, err := s.Queries.GetProfessionByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Profession{}, entity.ErrProfessionNotFound
		}
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
	const op = "repository.postgresql.profession.AddProfession"

	id, err := s.Queries.InsertProfession(ctx, postgresql.InsertProfessionParams{
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrUniqueViolation {
			return uuid.Nil, entity.ErrProfessionAlreadyExists
		}
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) UpdateProfession(ctx context.Context, profession entity.Profession) error {
	const op = "repository.postgresql.profession.UpdateProfession"

	ct, err := s.Pool.Exec(ctx,
		`UPDATE profession SET name = $2, vacancy_query = $3, is_active = $4 WHERE id = $1`,
		profession.ID, profession.Name, profession.VacancyQuery, profession.IsActive,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrUniqueViolation {
			return entity.ErrProfessionAlreadyExists
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	if ct.RowsAffected() == 0 {
		return entity.ErrProfessionNotFound
	}

	return nil
}
