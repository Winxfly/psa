package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"psa/internal/entity"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid token")
)

type UserProvider interface {
	GetUserByEmail(ctx context.Context, email string) (*entity.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

type RefreshTokenProvider interface {
	CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error
	GetRefreshToken(ctx context.Context, userID uuid.UUID, hashedToken string) (*entity.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, userID uuid.UUID, hashedToken string) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
}

type JWTProvider interface {
	GenerateAccessToken(ctx context.Context, userID uuid.UUID, role string) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error)
	ParseToken(ctx context.Context, token string) (*entity.TokenClaims, error)
	HashToken(token string) string
}

type Auth struct {
	log                  *slog.Logger
	userProvider         UserProvider
	refreshTokenProvider RefreshTokenProvider
	jwtProvider          JWTProvider
	accessTokenTTL       time.Duration
	refreshTokenTTL      time.Duration
}

func New(
	log *slog.Logger,
	userProvider UserProvider,
	refreshTokenProvider RefreshTokenProvider,
	jwtProvider JWTProvider,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
) *Auth {
	return &Auth{
		log:                  log,
		userProvider:         userProvider,
		refreshTokenProvider: refreshTokenProvider,
		jwtProvider:          jwtProvider,
		accessTokenTTL:       accessTokenTTL,
		refreshTokenTTL:      refreshTokenTTL,
	}
}

func (a *Auth) Signin(ctx context.Context, email, password string) (*entity.TokenPair, error) {
	const op = "usecase.auth.Signin"

	user, err := a.userProvider.GetUserByEmail(ctx, email)
	if err != nil {
		a.log.Error("Failed to get user by email", "email", email, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if user == nil {
		a.log.Warn("User not found", "email", email)
		return nil, ErrInvalidCredentials
	}

	if err := comparePassword(password, user.HashedPassword); err != nil {
		a.log.Warn("Invalid password", "email", email, "error", err)
		return nil, ErrInvalidCredentials
	}

	role := "user"
	if user.IsAdmin {
		role = "admin"
	}

	accessToken, err := a.jwtProvider.GenerateAccessToken(ctx, user.ID, role)
	if err != nil {
		a.log.Error("Failed to generate access token", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	refreshToken, err := a.jwtProvider.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		a.log.Error("Failed to generate refresh token", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	hashedRefreshToken := a.jwtProvider.HashToken(refreshToken)
	preparedRefreshToken := &entity.RefreshToken{
		UserID:      user.ID,
		HashedToken: hashedRefreshToken,
		ExpiresAt:   time.Now().Add(a.refreshTokenTTL),
	}

	if err := a.refreshTokenProvider.CreateRefreshToken(ctx, preparedRefreshToken); err != nil {
		a.log.Error("Failed to create refresh token", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("User signed in successfully", "user_id", user.ID, "email", email)

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *Auth) RefreshTokens(ctx context.Context, refreshToken string) (*entity.TokenPair, error) {
	const op = "usecase.auth.RefreshTokens"

	claims, err := a.jwtProvider.ParseToken(ctx, refreshToken)
	if err != nil {
		a.log.Warn("Failed to parse refresh token", "error", err)
		return nil, ErrInvalidToken
	}
	if claims.TokenType != entity.TokenTypeRefresh {
		a.log.Warn("Wrong token type for refresh", "expected", entity.TokenTypeRefresh, "got", claims.TokenType)
		return nil, ErrInvalidToken
	}

	user, err := a.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		a.log.Error("Failed to get user by ID", "user_id", claims.UserID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if user == nil {
		a.log.Warn("User not found for refresh", "user_id", claims.UserID)
		return nil, ErrUserNotFound
	}

	hashedToken := a.jwtProvider.HashToken(refreshToken)
	storedToken, err := a.refreshTokenProvider.GetRefreshToken(ctx, user.ID, hashedToken)
	if err != nil {
		a.log.Error("Failed to get refresh token from storage", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if storedToken == nil {
		a.log.Warn("Refresh token not found in storage", "user_id", user.ID)
		return nil, ErrInvalidToken
	}

	role := "user"
	if user.IsAdmin {
		role = "admin"
	}

	newAccessToken, err := a.jwtProvider.GenerateAccessToken(ctx, user.ID, role)
	if err != nil {
		a.log.Error("Failed to generate new access token", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	newRefreshToken, err := a.jwtProvider.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		a.log.Error("Failed to generate new refresh token", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	newHashedToken := a.jwtProvider.HashToken(newRefreshToken)
	preparedRefreshToken := &entity.RefreshToken{
		UserID:      user.ID,
		HashedToken: newHashedToken,
		ExpiresAt:   time.Now().Add(a.refreshTokenTTL),
	}

	if err := a.refreshTokenProvider.CreateRefreshToken(ctx, preparedRefreshToken); err != nil {
		a.log.Error("Failed to save refresh token", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := a.refreshTokenProvider.DeleteRefreshToken(ctx, user.ID, hashedToken); err != nil {
		a.log.Error("Failed to delete old refresh token", "user_id", user.ID, "error", err)
	}

	a.log.Info("Tokens refreshed successfully", "user_id", user.ID)

	return &entity.TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (a *Auth) Logout(ctx context.Context, refreshToken string) error {
	const op = "usecase.auth.Logout"

	claims, err := a.jwtProvider.ParseToken(ctx, refreshToken)
	if err != nil {
		a.log.Warn("Failed to parse token during logout", "error", err)
		return ErrInvalidToken
	}

	if claims.TokenType != entity.TokenTypeRefresh {
		a.log.Warn("Wrong token type for logout", "expected", entity.TokenTypeRefresh, "got", claims.TokenType)
		return ErrInvalidToken
	}

	hashedToken := a.jwtProvider.HashToken(refreshToken)

	if err := a.refreshTokenProvider.DeleteRefreshToken(ctx, claims.UserID, hashedToken); err != nil {
		a.log.Error("Failed to delete old refresh token during logout", "user_id", claims.UserID, "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("User logged out successfully", "user_id", claims.UserID)

	return nil
}

func (a *Auth) ValidateToken(ctx context.Context, token string) (*entity.TokenClaims, error) {
	const op = "usecase.auth.ValidateToken"

	claims, err := a.jwtProvider.ParseToken(ctx, token)
	if err != nil {
		a.log.Warn("Failed to parse token during validation", "error", err)
		return nil, ErrInvalidToken
	}

	if claims.TokenType != entity.TokenTypeAccess {
		a.log.Warn("Wrong token type for validation", "expected", entity.TokenTypeAccess, "got", claims.TokenType)
		return nil, ErrInvalidToken
	}

	user, err := a.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		a.log.Error("Failed to get user during token validation", "user_id", claims.UserID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if user == nil {
		a.log.Warn("User not found for validation", "user_id", claims.UserID)
		return nil, ErrUserNotFound
	}

	a.log.Debug("Token validated successful", "user_id", claims.UserID)

	return claims, nil
}

func comparePassword(password string, comparedHashedPassword string) error {
	hashedPassword, err := base64.StdEncoding.DecodeString(comparedHashedPassword)
	if err == nil {
		err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
		if err != nil {
			return fmt.Errorf("invalid password: %w", err)
		}

		return nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(comparedHashedPassword), []byte(password))
	if err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	return nil
}
