package auth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"psa/internal/domain"
	"psa/internal/service/auth"
	"psa/internal/service/auth/mocks"
)

// hashPassword создаёт bcrypt хеш и кодирует его в base64 (как в реальном приложении)
func hashPassword(t *testing.T, password string) string {
	t.Helper()

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(hashed)
}

func newTestAuth(
	userProvider *mocks.MockUserProvider,
	refreshTokenProvider *mocks.MockRefreshTokenProvider,
	jwtProvider *mocks.MockJWTProvider,
) *auth.Auth {
	return auth.New(
		userProvider,
		refreshTokenProvider,
		jwtProvider,
		15*time.Minute,
		168*time.Hour,
	)
}

// testDeps содержит зависимости для тестирования Auth
type testDeps struct {
	user  *mocks.MockUserProvider
	token *mocks.MockRefreshTokenProvider
	jwt   *mocks.MockJWTProvider
}

func newDeps(t *testing.T) testDeps {
	t.Helper()
	return testDeps{
		user:  mocks.NewMockUserProvider(t),
		token: mocks.NewMockRefreshTokenProvider(t),
		jwt:   mocks.NewMockJWTProvider(t),
	}
}

func (d testDeps) auth() *auth.Auth {
	return newTestAuth(d.user, d.token, d.jwt)
}

func TestAuth_Signin_Unit_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "test@example.com"
	password := "password123"

	// Arrange
	deps := newDeps(t)

	user := &domain.User{
		ID:             uuid.New(),
		Email:          email,
		HashedPassword: hashPassword(t, password),
		IsAdmin:        false,
	}

	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(user, nil)
	deps.jwt.EXPECT().GenerateAccessToken(ctx, user.ID, "user").Return("access-token", nil)
	deps.jwt.EXPECT().GenerateRefreshToken(ctx, user.ID).Return("refresh-token", nil)
	deps.jwt.EXPECT().HashToken("refresh-token").Return("hashed-refresh-token")
	deps.token.EXPECT().CreateRefreshToken(ctx, mock.MatchedBy(func(token *domain.RefreshToken) bool {
		return token.UserID == user.ID && token.HashedToken == "hashed-refresh-token"
	})).Return(nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, password)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, tokens)
	assert.Equal(t, "access-token", tokens.AccessToken)
	assert.Equal(t, "refresh-token", tokens.RefreshToken)
}

func TestAuth_Signin_Unit_UserNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "nonexistent@example.com"

	// Arrange
	deps := newDeps(t)

	// Сервис возвращает ErrInvalidCredentials, когда GetUserByEmail возвращает nil без ошибки
	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(nil, nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, "password")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuth_Signin_Unit_InvalidPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "test@example.com"
	wrongPassword := "wrong-password"
	correctPassword := "password123"

	// Arrange
	deps := newDeps(t)

	user := &domain.User{
		ID:             uuid.New(),
		Email:          email,
		HashedPassword: hashPassword(t, correctPassword),
		IsAdmin:        false,
	}

	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(user, nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, wrongPassword)

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	deps.jwt.AssertNotCalled(t, "GenerateAccessToken")
	deps.jwt.AssertNotCalled(t, "GenerateRefreshToken")
	deps.token.AssertNotCalled(t, "CreateRefreshToken")
}

func TestAuth_Signin_Unit_GetUserError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "test@example.com"

	// Arrange
	deps := newDeps(t)

	dbError := errors.New("database connection failed")
	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(nil, dbError)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, "password")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, dbError)
}

func TestAuth_Signin_Unit_GenerateAccessTokenError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "test@example.com"
	password := "password123"
	tokenErr := errors.New("token generation failed")

	// Arrange
	deps := newDeps(t)

	user := &domain.User{
		ID:             uuid.New(),
		Email:          email,
		HashedPassword: hashPassword(t, password),
		IsAdmin:        false,
	}

	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(user, nil)
	deps.jwt.EXPECT().GenerateAccessToken(ctx, user.ID, "user").Return("", tokenErr)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, password)

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, tokenErr)
	deps.jwt.AssertNotCalled(t, "GenerateRefreshToken")
	deps.token.AssertNotCalled(t, "CreateRefreshToken")
}

func TestAuth_Signin_Unit_AdminUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "admin@example.com"
	password := "password123"

	// Arrange
	deps := newDeps(t)

	user := &domain.User{
		ID:             uuid.New(),
		Email:          email,
		HashedPassword: hashPassword(t, password),
		IsAdmin:        true,
	}

	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(user, nil)
	deps.jwt.EXPECT().GenerateAccessToken(ctx, user.ID, "admin").Return("admin-access-token", nil)
	deps.jwt.EXPECT().GenerateRefreshToken(ctx, user.ID).Return("admin-refresh-token", nil)
	deps.jwt.EXPECT().HashToken("admin-refresh-token").Return("hashed-refresh-token")
	deps.token.EXPECT().CreateRefreshToken(ctx, mock.MatchedBy(func(token *domain.RefreshToken) bool {
		return token.UserID == user.ID
	})).Return(nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, password)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, tokens)
	assert.Equal(t, "admin-access-token", tokens.AccessToken)
}

// ==================== RefreshTokens ====================

func TestAuth_RefreshTokens_Unit_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := uuid.New()

	// Arrange
	deps := newDeps(t)

	user := &domain.User{
		ID:      userID,
		Email:   "test@example.com",
		IsAdmin: false,
	}

	refreshToken := "valid-refresh-token"
	hashedToken := "hashed-refresh-token"
	newRefreshToken := "new-refresh-token"
	newHashedToken := "new-hashed-token"

	deps.jwt.EXPECT().ParseToken(ctx, refreshToken).Return(&domain.TokenClaims{
		TokenType: "refresh_token",
		UserID:    userID,
	}, nil)
	deps.jwt.EXPECT().HashToken(refreshToken).Return(hashedToken)
	deps.user.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	deps.token.EXPECT().GetRefreshToken(ctx, userID, hashedToken).Return(&domain.RefreshToken{
		UserID:      userID,
		HashedToken: hashedToken,
	}, nil)
	deps.jwt.EXPECT().GenerateAccessToken(ctx, userID, "user").Return("new-access-token", nil)
	deps.jwt.EXPECT().GenerateRefreshToken(ctx, userID).Return(newRefreshToken, nil)
	deps.jwt.EXPECT().HashToken(newRefreshToken).Return(newHashedToken)

	// Порядок вызовов важен: сначала Delete, потом Create
	deps.token.EXPECT().DeleteRefreshToken(ctx, userID, hashedToken).Return(nil)
	deps.token.EXPECT().CreateRefreshToken(ctx, mock.MatchedBy(func(token *domain.RefreshToken) bool {
		return token.UserID == userID && token.HashedToken == newHashedToken
	})).Return(nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.RefreshTokens(ctx, refreshToken)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, tokens)
	assert.Equal(t, "new-access-token", tokens.AccessToken)
	assert.Equal(t, newRefreshToken, tokens.RefreshToken)
}

func TestAuth_RefreshTokens_Unit_InvalidToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "invalid-token").Return(nil, errors.New("invalid token"))

	authUC := deps.auth()

	// Act
	tokens, err := authUC.RefreshTokens(ctx, "invalid-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.user.AssertNotCalled(t, "GetUserByID")
	deps.token.AssertNotCalled(t, "GetRefreshToken")
	deps.token.AssertNotCalled(t, "DeleteRefreshToken")
	deps.token.AssertNotCalled(t, "CreateRefreshToken")
}

func TestAuth_RefreshTokens_Unit_WrongTokenType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "access-token").Return(&domain.TokenClaims{
		TokenType: "access_token", // Wrong type
	}, nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.RefreshTokens(ctx, "access-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.user.AssertNotCalled(t, "GetUserByID")
	deps.token.AssertNotCalled(t, "GetRefreshToken")
	deps.token.AssertNotCalled(t, "DeleteRefreshToken")
	deps.token.AssertNotCalled(t, "CreateRefreshToken")
}

func TestAuth_RefreshTokens_Unit_UserNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := uuid.New()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "refresh-token").Return(&domain.TokenClaims{
		TokenType: "refresh_token",
		UserID:    userID,
	}, nil)
	deps.user.EXPECT().GetUserByID(ctx, userID).Return(nil, auth.ErrUserNotFound)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.RefreshTokens(ctx, "refresh-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, auth.ErrUserNotFound)
	deps.jwt.AssertNotCalled(t, "GenerateAccessToken")
	deps.jwt.AssertNotCalled(t, "GenerateRefreshToken")
	deps.token.AssertNotCalled(t, "GetRefreshToken")
	deps.token.AssertNotCalled(t, "DeleteRefreshToken")
	deps.token.AssertNotCalled(t, "CreateRefreshToken")
}

func TestAuth_RefreshTokens_Unit_TokenNotFoundInDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := uuid.New()

	// Arrange
	deps := newDeps(t)

	refreshToken := "refresh-token"
	hashedToken := "hashed-token"

	deps.jwt.EXPECT().ParseToken(ctx, refreshToken).Return(&domain.TokenClaims{
		TokenType: "refresh_token",
		UserID:    userID,
	}, nil)
	deps.jwt.EXPECT().HashToken(refreshToken).Return(hashedToken)
	deps.user.EXPECT().GetUserByID(ctx, userID).Return(&domain.User{ID: userID}, nil)
	deps.token.EXPECT().GetRefreshToken(ctx, userID, hashedToken).Return(nil, nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.RefreshTokens(ctx, refreshToken)

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.jwt.AssertNotCalled(t, "GenerateAccessToken")
	deps.jwt.AssertNotCalled(t, "GenerateRefreshToken")
	deps.token.AssertNotCalled(t, "DeleteRefreshToken")
	deps.token.AssertNotCalled(t, "CreateRefreshToken")
}

// ==================== Logout ====================

func TestAuth_Logout_Unit_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := uuid.New()

	// Arrange
	deps := newDeps(t)

	refreshToken := "refresh-token"
	hashedToken := "hashed-token"

	deps.jwt.EXPECT().ParseToken(ctx, refreshToken).Return(&domain.TokenClaims{
		TokenType: "refresh_token",
		UserID:    userID,
	}, nil)
	deps.jwt.EXPECT().HashToken(refreshToken).Return(hashedToken)
	deps.token.EXPECT().DeleteRefreshToken(ctx, userID, hashedToken).Return(nil)

	authUC := deps.auth()

	// Act
	err := authUC.Logout(ctx, refreshToken)

	// Assert
	require.NoError(t, err)
}

func TestAuth_Logout_Unit_InvalidToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "invalid-token").Return(nil, errors.New("invalid token"))

	authUC := deps.auth()

	// Act
	err := authUC.Logout(ctx, "invalid-token")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.token.AssertNotCalled(t, "DeleteRefreshToken")
}

func TestAuth_Logout_Unit_WrongTokenType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "access-token").Return(&domain.TokenClaims{
		TokenType: "access_token", // Wrong type
	}, nil)

	authUC := deps.auth()

	// Act
	err := authUC.Logout(ctx, "access-token")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.token.AssertNotCalled(t, "DeleteRefreshToken")
}

// ==================== ValidateToken ====================

func TestAuth_ValidateToken_Unit_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := uuid.New()

	// Arrange
	deps := newDeps(t)

	user := &domain.User{
		ID:      userID,
		Email:   "test@example.com",
		IsAdmin: false,
	}

	deps.jwt.EXPECT().ParseToken(ctx, "access-token").Return(&domain.TokenClaims{
		TokenType: "access_token",
		UserID:    userID,
		Role:      "user",
	}, nil)
	deps.user.EXPECT().GetUserByID(ctx, userID).Return(user, nil)

	authUC := deps.auth()

	// Act
	claims, err := authUC.ValidateToken(ctx, "access-token")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "user", claims.Role)
}

func TestAuth_ValidateToken_Unit_InvalidToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "invalid-token").Return(nil, errors.New("invalid token"))

	authUC := deps.auth()

	// Act
	claims, err := authUC.ValidateToken(ctx, "invalid-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, claims)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.user.AssertNotCalled(t, "GetUserByID")
}

func TestAuth_ValidateToken_Unit_WrongTokenType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "refresh-token").Return(&domain.TokenClaims{
		TokenType: "refresh_token", // Wrong type
	}, nil)

	authUC := deps.auth()

	// Act
	claims, err := authUC.ValidateToken(ctx, "refresh-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, claims)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	deps.user.AssertNotCalled(t, "GetUserByID")
}

func TestAuth_ValidateToken_Unit_UserNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := uuid.New()

	// Arrange
	deps := newDeps(t)

	deps.jwt.EXPECT().ParseToken(ctx, "access-token").Return(&domain.TokenClaims{
		TokenType: "access_token",
		UserID:    userID,
	}, nil)
	deps.user.EXPECT().GetUserByID(ctx, userID).Return(nil, auth.ErrUserNotFound)

	authUC := deps.auth()

	// Act
	claims, err := authUC.ValidateToken(ctx, "access-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, claims)
	assert.ErrorIs(t, err, auth.ErrUserNotFound)
}

// ==================== comparePassword (через Signin) ====================

func TestAuth_Signin_Unit_ValidPassword_Base64Encoded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "test@example.com"
	password := "securePassword123"

	// Arrange
	deps := newDeps(t)

	// Создаём настоящий bcrypt хеш и кодируем в base64 (как в реальном приложении)
	hashedPassword := hashPassword(t, password)

	user := &domain.User{
		ID:             uuid.New(),
		Email:          email,
		HashedPassword: hashedPassword,
		IsAdmin:        false,
	}

	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(user, nil)
	deps.jwt.EXPECT().GenerateAccessToken(ctx, user.ID, "user").Return("access-token", nil)
	deps.jwt.EXPECT().GenerateRefreshToken(ctx, user.ID).Return("refresh-token", nil)
	deps.jwt.EXPECT().HashToken("refresh-token").Return("hashed-refresh-token")
	deps.token.EXPECT().CreateRefreshToken(ctx, mock.MatchedBy(func(token *domain.RefreshToken) bool {
		return token.UserID == user.ID
	})).Return(nil)

	authUC := deps.auth()

	// Act
	tokens, err := authUC.Signin(ctx, email, password)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, tokens)
	assert.Equal(t, "access-token", tokens.AccessToken)
}

// TestAuth_ComparePassword_Unit_PlainBcrypt тестирует ветку с обычным bcrypt (не base64)
// Это нужно для покрытия функции comparePassword и поддержки легаси хешей
func TestAuth_ComparePassword_Unit_PlainBcrypt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	email := "legacy@example.com"
	password := "legacyPassword"

	// Arrange
	deps := newDeps(t)

	// Создаём настоящий bcrypt хеш БЕЗ base64 кодирования (легаси формат)
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	user := &domain.User{
		ID:             uuid.New(),
		Email:          email,
		HashedPassword: string(hashed), // Не base64!
		IsAdmin:        false,
	}

	deps.user.EXPECT().GetUserByEmail(ctx, email).Return(user, nil)
	deps.jwt.EXPECT().GenerateAccessToken(ctx, user.ID, "user").Return("access-token", nil)
	deps.jwt.EXPECT().GenerateRefreshToken(ctx, user.ID).Return("refresh-token", nil)
	deps.jwt.EXPECT().HashToken("refresh-token").Return("hashed-refresh-token")
	deps.token.EXPECT().CreateRefreshToken(ctx, mock.MatchedBy(func(token *domain.RefreshToken) bool {
		return token.UserID == user.ID
	})).Return(nil)

	authUC := deps.auth()

	// Act
	tokens, signinErr := authUC.Signin(ctx, email, password)

	// Assert
	require.NoError(t, signinErr)
	require.NotNil(t, tokens)
	assert.Equal(t, "access-token", tokens.AccessToken)
}
