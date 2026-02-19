package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"psa/internal/domain"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid token")
)

type UserProvider interface {
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type RefreshTokenProvider interface {
	CreateRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, userID uuid.UUID, hashedToken string) (*domain.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, userID uuid.UUID, hashedToken string) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
}

type JWTProvider interface {
	GenerateAccessToken(ctx context.Context, userID uuid.UUID, role string) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error)
	ParseToken(ctx context.Context, token string) (*domain.TokenClaims, error)
	HashToken(token string) string
}

type Auth struct {
	userProvider         UserProvider
	refreshTokenProvider RefreshTokenProvider
	jwtProvider          JWTProvider
	accessTokenTTL       time.Duration
	refreshTokenTTL      time.Duration
}

func New(
	userProvider UserProvider,
	refreshTokenProvider RefreshTokenProvider,
	jwtProvider JWTProvider,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
) *Auth {
	return &Auth{
		userProvider:         userProvider,
		refreshTokenProvider: refreshTokenProvider,
		jwtProvider:          jwtProvider,
		accessTokenTTL:       accessTokenTTL,
		refreshTokenTTL:      refreshTokenTTL,
	}
}

func (a *Auth) Signin(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	const op = "service.auth.Signin"
	log := loggerctx.FromContext(ctx).With("op", op)

	user, err := a.userProvider.GetUserByEmail(ctx, email)
	if err != nil {
		log.Error("get_user_failed", slogx.Err(err))
		return nil, fmt.Errorf("%s: get user: %w", op, err)
	}
	if user == nil {
		log.Warn("user_not_found")
		return nil, ErrInvalidCredentials
	}

	if err := comparePassword(password, user.HashedPassword); err != nil {
		log.Warn("invalid_password", "user_id", user.ID, slogx.Err(err))
		return nil, ErrInvalidCredentials
	}

	role := "user"
	if user.IsAdmin {
		role = "admin"
	}

	accessToken, err := a.jwtProvider.GenerateAccessToken(ctx, user.ID, role)
	if err != nil {
		log.Error("generate_access_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: generate access token: %w", op, err)
	}

	refreshToken, err := a.jwtProvider.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		log.Error("generate_refresh_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: generate refresh token: %w", op, err)
	}

	hashedRefreshToken := a.jwtProvider.HashToken(refreshToken)
	preparedRefreshToken := &domain.RefreshToken{
		UserID:      user.ID,
		HashedToken: hashedRefreshToken,
		ExpiresAt:   time.Now().Add(a.refreshTokenTTL),
	}

	if err := a.refreshTokenProvider.CreateRefreshToken(ctx, preparedRefreshToken); err != nil {
		log.Error("create_refresh_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: create refresh token: %w", op, err)
	}

	log.Info("signin_success", "user_id", user.ID, "role", role)

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *Auth) RefreshTokens(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	const op = "service.auth.RefreshTokens"
	log := loggerctx.FromContext(ctx).With("op", op)

	claims, err := a.jwtProvider.ParseToken(ctx, refreshToken)
	if err != nil {
		log.Warn("parse_failed", slogx.Err(err))
		return nil, ErrInvalidToken
	}
	if claims.TokenType != domain.TokenTypeRefresh {
		log.Warn("wrong_token_type", "expected", domain.TokenTypeRefresh, "got", claims.TokenType)
		return nil, ErrInvalidToken
	}

	user, err := a.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		log.Error("get_user_failed", "user_id", claims.UserID, slogx.Err(err))
		return nil, fmt.Errorf("%s: get user: %w", op, err)
	}
	if user == nil {
		log.Warn("user_not_found", "user_id", claims.UserID)
		return nil, ErrUserNotFound
	}

	hashedToken := a.jwtProvider.HashToken(refreshToken)
	storedToken, err := a.refreshTokenProvider.GetRefreshToken(ctx, user.ID, hashedToken)
	if err != nil {
		log.Error("get_stored_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: get stored token: %w", op, err)
	}
	if storedToken == nil {
		log.Warn("token_not_found", "user_id", user.ID)
		return nil, ErrInvalidToken
	}

	role := "user"
	if user.IsAdmin {
		role = "admin"
	}

	newAccessToken, err := a.jwtProvider.GenerateAccessToken(ctx, user.ID, role)
	if err != nil {
		log.Error("generate_access_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: generate access token: %w", op, err)
	}

	newRefreshToken, err := a.jwtProvider.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		log.Error("generate_refresh_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: generate refresh token: %w", op, err)
	}

	newHashedToken := a.jwtProvider.HashToken(newRefreshToken)
	preparedRefreshToken := &domain.RefreshToken{
		UserID:      user.ID,
		HashedToken: newHashedToken,
		ExpiresAt:   time.Now().Add(a.refreshTokenTTL),
	}

	if err := a.refreshTokenProvider.CreateRefreshToken(ctx, preparedRefreshToken); err != nil {
		log.Error("create_refresh_token_failed", "user_id", user.ID, slogx.Err(err))
		return nil, fmt.Errorf("%s: create refresh token: %w", op, err)
	}

	if err := a.refreshTokenProvider.DeleteRefreshToken(ctx, user.ID, hashedToken); err != nil {
		log.Warn("delete_old_token_failed", "user_id", user.ID, slogx.Err(err))
	}

	log.Info("refresh_tokens_success", "user_id", user.ID)

	return &domain.TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (a *Auth) Logout(ctx context.Context, refreshToken string) error {
	const op = "service.auth.Logout"
	log := loggerctx.FromContext(ctx).With("op", op)

	claims, err := a.jwtProvider.ParseToken(ctx, refreshToken)
	if err != nil {
		log.Warn("parse_failed", slogx.Err(err))
		return ErrInvalidToken
	}

	if claims.TokenType != domain.TokenTypeRefresh {
		log.Warn("wrong_token_type", "expected", domain.TokenTypeRefresh, "got", claims.TokenType)
		return ErrInvalidToken
	}

	hashedToken := a.jwtProvider.HashToken(refreshToken)

	if err := a.refreshTokenProvider.DeleteRefreshToken(ctx, claims.UserID, hashedToken); err != nil {
		log.Error("delete_token_failed", "user_id", claims.UserID, slogx.Err(err))
		return fmt.Errorf("%s: delete token: %w", op, err)
	}

	log.Info("logout_success", "user_id", claims.UserID)

	return nil
}

func (a *Auth) ValidateToken(ctx context.Context, token string) (*domain.TokenClaims, error) {
	const op = "service.auth.ValidateToken"
	log := loggerctx.FromContext(ctx).With("op", op)

	claims, err := a.jwtProvider.ParseToken(ctx, token)
	if err != nil {
		log.Warn("parse_failed", slogx.Err(err))
		return nil, ErrInvalidToken
	}

	if claims.TokenType != domain.TokenTypeAccess {
		log.Warn("wrong_token_type", "expected", domain.TokenTypeAccess, "got", claims.TokenType)
		return nil, ErrInvalidToken
	}

	user, err := a.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		log.Error("get_user_failed", "user_id", claims.UserID, slogx.Err(err))
		return nil, fmt.Errorf("%s: get user: %w", op, err)
	}
	if user == nil {
		log.Warn("user_not_found", "user_id", claims.UserID)
		return nil, ErrUserNotFound
	}

	log.Debug("validate_token_success", "user_id", claims.UserID, "role", claims.Role)

	return claims, nil
}

func comparePassword(password string, comparedHashedPassword string) error {
	const op = "service.auth.comparePassword"

	hashedPassword, err := base64.StdEncoding.DecodeString(comparedHashedPassword)
	if err == nil {
		err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
		if err != nil {
			return fmt.Errorf("%s: invalid password: %w", op, err)
		}

		return nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(comparedHashedPassword), []byte(password))
	if err != nil {
		return fmt.Errorf("%s: invalid password: %w", op, err)
	}

	return nil
}
