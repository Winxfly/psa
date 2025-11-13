package auth

import (
	"context"
	"github.com/google/uuid"
	"psa/internal/entity"
	"psa/pkg/jwtmanager"
)

type JWTAdapter struct {
	manager *jwtmanager.Manager
}

func NewJWTAdapter(manager *jwtmanager.Manager) *JWTAdapter {
	return &JWTAdapter{manager: manager}
}

func (a *JWTAdapter) GenerateAccessToken(ctx context.Context, userID uuid.UUID, role string) (string, error) {
	return a.manager.GenerateAccessToken(userID, role)
}

func (a *JWTAdapter) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	return a.manager.GenerateRefreshToken(userID)
}

func (a *JWTAdapter) ParseToken(ctx context.Context, token string) (*entity.TokenClaims, error) {
	jwtClaims, err := a.manager.ParseToken(token)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(jwtClaims.Subject)
	if err != nil {
		return nil, err
	}

	return &entity.TokenClaims{
		UserID:    userID,
		Role:      jwtClaims.Role,
		TokenType: entity.TokenType(jwtClaims.TokenType),
		ExpiresAt: jwtClaims.ExpiresAt.Time,
	}, nil
}

func (a *JWTAdapter) ExtractUserID(ctx context.Context, token string) (uuid.UUID, error) {
	return a.manager.ExtractUserID(token)
}

func (a *JWTAdapter) HashToken(token string) string {
	return jwtmanager.HashToken(token)
}
