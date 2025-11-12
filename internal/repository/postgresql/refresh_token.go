package postgresql

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"psa/internal/entity"
	postgresql "psa/internal/repository/postgresql/generated"
)

func (s *Storage) CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	const op = "repository.postgresql.refresh_token.CreateRefreshToken"

	_, err := s.Queries.InsertRefreshToken(ctx, postgresql.InsertRefreshTokenParams{
		UserID:      token.UserID,
		HashedToken: token.HashedToken,
		ExpiresAt:   token.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) GetRefreshToken(ctx context.Context, userID uuid.UUID, hashedToken string) (*entity.RefreshToken, error) {
	const op = "repository.postgresql.refresh_token.GetRefreshToken"

	token, err := s.Queries.GetRefreshToken(ctx, postgresql.GetRefreshTokenParams{
		UserID:      userID,
		HashedToken: hashedToken,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &entity.RefreshToken{
		UserID:      token.UserID,
		HashedToken: token.HashedToken,
		CreatedAt:   token.CreatedAt,
		ExpiresAt:   token.ExpiresAt,
	}, nil
}

func (s *Storage) DeleteRefreshToken(ctx context.Context, userID uuid.UUID, hashedToken string) error {
	const op = "repository.postgresql.refresh_token.DeleteRefreshToken"

	err := s.Queries.DeleteRefreshToken(ctx, postgresql.DeleteRefreshTokenParams{
		UserID:      userID,
		HashedToken: hashedToken,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) DeleteExpiredRefreshTokens(ctx context.Context) error {
	return s.Queries.DeleteExpiredRefreshTokens(ctx)
}
