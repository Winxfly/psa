package auth

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/entity"
	jwtpkg "psa/pkg/jwt"
)

type JWTAdapter struct {
	manager *jwtpkg.Manager
}

func NewJWTAdapter(manager *jwtpkg.Manager) *JWTAdapter {
	return &JWTAdapter{manager: manager}
}

func (a *JWTAdapter) GenerateAccessToken(ctx context.Context, userID uuid.UUID, role string) (string, error) {
	return a.manager.GenerateAccessToken(userID, role)
}

func (a *JWTAdapter) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	return a.manager.GenerateRefreshToken(userID)
}

func (a *JWTAdapter) ParseToken(ctx context.Context, token string) (*entity.TokenClaims, error) {
	const op = "auth.JWTAdapter.ParseToken"

	jwtClaims, err := a.manager.ParseToken(token)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	userID, err := uuid.Parse(jwtClaims.Subject)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid user id: %w", op, err)
	}

	var tokenType entity.TokenType
	switch jwtClaims.TokenType {
	case "access_token":
		tokenType = entity.TokenTypeAccess
	case "refresh_token":
		tokenType = entity.TokenTypeRefresh
	default:
		return nil, fmt.Errorf("%s: unknown token type: %s", op, jwtClaims.TokenType)
	}

	return &entity.TokenClaims{
		UserID:    userID,
		Role:      jwtClaims.Role,
		TokenType: tokenType,
		ExpiresAt: jwtClaims.ExpiresAt.Time,
	}, nil
}

func (a *JWTAdapter) ExtractUserID(ctx context.Context, token string) (uuid.UUID, error) {
	return a.manager.ExtractUserID(token)
}

func (a *JWTAdapter) HashToken(token string) string {
	return jwtpkg.HashToken(token)
}
