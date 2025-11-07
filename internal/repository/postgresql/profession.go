package postgresql

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/entity"
	postgresql "psa/internal/repository/postgresql/generated"
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

	return s.Queries.InsertProfession(ctx, postgresql.InsertProfessionParams{
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})
}

func (s *Storage) UpdateProfession(ctx context.Context, profession entity.Profession) error {
	const op = "repository.postgresql.profession.UpdateProfession"

	return s.Queries.UpdateProfession(ctx, postgresql.UpdateProfessionParams{
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})
}
