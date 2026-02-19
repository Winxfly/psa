package postgresql

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"psa/internal/domain"
)

func (s *Storage) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	const op = "repository.postgresql.user.GetUserByEmail"

	user, err := s.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &domain.User{
		ID:             user.ID,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
		IsAdmin:        user.IsAdmin,
		CreatedAt:      user.CreatedAt.Time,
	}, nil
}

func (s *Storage) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const op = "repository.postgresql.user.GetUserByID"

	user, err := s.Queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &domain.User{
		ID:             user.ID,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
		IsAdmin:        user.IsAdmin,
		CreatedAt:      user.CreatedAt.Time,
	}, nil
}
