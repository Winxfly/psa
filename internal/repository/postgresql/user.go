package postgresql

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/entity"
)

func (s *Storage) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	const op = "repository.postgresql.user.GetUserByEmail"

	user, err := s.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &entity.User{
		ID:             user.ID,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
		IsAdmin:        user.IsAdmin,
		CreatedAt:      user.CreatedAt.Time,
	}, nil
}

func (s *Storage) GetUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	const op = "repository.postgresql.user.GetUserByID"

	user, err := s.Queries.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &entity.User{
		ID:             user.ID,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
		IsAdmin:        user.IsAdmin,
		CreatedAt:      user.CreatedAt.Time,
	}, nil
}
