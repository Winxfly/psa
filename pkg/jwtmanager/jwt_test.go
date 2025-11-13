package jwtmanager_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"psa/pkg/jwtmanager"
	"strings"
	"testing"
	"time"
)

func setupManager(secret string, accessTTL, refreshTTL time.Duration) *jwtmanager.Manager {
	return jwtmanager.NewJWT(secret, accessTTL, refreshTTL, "test-issuer")
}

func TestJWTManager_TokenLifecycle(t *testing.T) {
	tests := []struct {
		name            string
		accessTokenTTL  time.Duration
		refreshTokenTTL time.Duration
		issuer          string
		role            string
	}{
		{
			name:            "standard_case",
			accessTokenTTL:  time.Minute * 15,
			refreshTokenTTL: time.Hour * 24 * 7,
			issuer:          "psa-service",
			role:            "admin",
		},
		{
			name:            "short_lived_access_token",
			accessTokenTTL:  time.Second * 30,
			refreshTokenTTL: time.Hour,
			issuer:          "test-suite",
			role:            "user",
		},
		{
			name:            "different_issuer_and_role",
			accessTokenTTL:  time.Minute * 5,
			refreshTokenTTL: time.Hour * 2,
			issuer:          "auth-api",
			role:            "manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := jwtmanager.NewJWT(
				"secret-key-"+tt.name,
				tt.accessTokenTTL,
				tt.refreshTokenTTL,
				tt.issuer,
			)

			userID := uuid.New()

			accessToken, err := manager.GenerateAccessToken(userID, tt.role)
			require.NoError(t, err)
			require.NotEmpty(t, accessToken)

			refreshToken, err := manager.GenerateRefreshToken(userID)
			require.NoError(t, err)
			require.NotEmpty(t, refreshToken)

			require.NotEqual(t, accessToken, refreshToken)

			claims, err := manager.ParseToken(accessToken)
			require.NoError(t, err)
			require.Equal(t, "access_token", claims.TokenType)
			require.Equal(t, tt.role, claims.Role)
			require.Equal(t, tt.issuer, claims.Issuer)
			require.Equal(t, userID.String(), claims.Subject)
			require.True(t, claims.IsAccessToken())
			require.False(t, claims.IsRefreshToken())

			refreshClaims, err := manager.ParseToken(refreshToken)
			require.NoError(t, err)
			require.Equal(t, "refresh_token", refreshClaims.TokenType)
			require.Equal(t, userID.String(), refreshClaims.Subject)
			require.Equal(t, tt.issuer, refreshClaims.Issuer)
			require.True(t, refreshClaims.IsRefreshToken())
			require.False(t, refreshClaims.IsAccessToken())

			extractedID, err := manager.ExtractUserID(accessToken)
			require.NoError(t, err)
			require.Equal(t, userID, extractedID)
		})
	}
}

func TestJWTManager_TokenTimestamps(t *testing.T) {
	manager := setupManager("secret", time.Minute*15, time.Hour*24)
	userID := uuid.New()

	before := time.Now()
	token, err := manager.GenerateAccessToken(userID, "user")
	require.NoError(t, err)
	after := time.Now()

	claims, err := manager.ParseToken(token)
	require.NoError(t, err)

	require.True(t, claims.IssuedAt.Time.After(before.Add(-time.Second)) || claims.IssuedAt.Time.Equal(before))
	require.True(t, claims.IssuedAt.Time.Before(after.Add(time.Second)) || claims.IssuedAt.Time.Equal(after))

	expectedExpiry := claims.IssuedAt.Time.Add(time.Minute * 15)
	require.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, time.Second)
}

func TestJWTManager_DifferentRoles(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)
	userID := uuid.New()

	roles := []string{"user", "admin", "moderator", ""}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			token, err := manager.GenerateAccessToken(userID, role)
			require.NoError(t, err)

			claims, err := manager.ParseToken(token)
			require.NoError(t, err)
			require.Equal(t, role, claims.Role)
		})
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	manager := setupManager("secret", -time.Minute, -time.Hour)

	userID := uuid.New()
	token, err := manager.GenerateAccessToken(userID, "user")
	require.NoError(t, err)

	_, err = manager.ParseToken(token)
	require.Error(t, err)
	require.True(t, errors.Is(err, jwt.ErrTokenExpired))
}

func TestJWTManager_ParseToken_InvalidSignature(t *testing.T) {
	manager1 := setupManager("secret1", time.Minute, time.Hour)
	manager2 := setupManager("wrongsecret", time.Minute, time.Hour)

	userID := uuid.New()
	token, err := manager1.GenerateAccessToken(userID, "user")
	require.NoError(t, err)

	_, err = manager2.ParseToken(token)
	require.Error(t, err)
	require.True(t, errors.Is(err, jwt.ErrTokenSignatureInvalid))
}

func TestJWTManager_InvalidSigningMethod(t *testing.T) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &jwtmanager.TokenClaims{
		TokenType: "access_token",
		Role:      "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "test",
			Subject:   uuid.New().String(),
		},
	})

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signedToken, err := token.SignedString(key)
	require.NoError(t, err)

	manager := setupManager("secret", time.Minute, time.Hour)
	_, err = manager.ParseToken(signedToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected signing method")
}

func TestJWTManager_ExtractUserID_InvalidToken(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)

	tests := []struct {
		name          string
		token         string
		expectedError error
	}{
		{"empty_token", "", jwt.ErrTokenMalformed},
		{"malformed_token", "not.a.jwtmanager.token", jwt.ErrTokenMalformed},
		{"invalid_format", "invalid.token.value", jwt.ErrTokenMalformed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ExtractUserID(tt.token)
			require.Error(t, err)
			require.True(t, errors.Is(err, tt.expectedError))
		})
	}
}

func TestJWTManager_InvalidUserIDInToken(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwtmanager.TokenClaims{
		TokenType: "access_token",
		Role:      "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "test",
			Subject:   "invalid-uuid-format",
		},
	})
	signedToken, err := token.SignedString([]byte("secret"))
	require.NoError(t, err)

	_, err = manager.ExtractUserID(signedToken)
	require.Error(t, err)
	require.True(t, errors.Is(err, jwtmanager.ErrInvalidUserID))
}

func TestJWTManager_EdgeCases(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)

	t.Run("nil_uuid", func(t *testing.T) {
		token, err := manager.GenerateAccessToken(uuid.Nil, "user")
		require.NoError(t, err)

		claims, err := manager.ParseToken(token)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil.String(), claims.Subject)
	})

	t.Run("very_long_secret", func(t *testing.T) {
		longSecret := strings.Repeat("a", 1000)
		manager := setupManager(longSecret, time.Minute, time.Hour)

		userID := uuid.New()
		token, err := manager.GenerateAccessToken(userID, "user")
		require.NoError(t, err)

		_, err = manager.ParseToken(token)
		require.NoError(t, err)
	})
}

func TestHashToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"standard_token", "example.jwtmanager.token"},
		{"empty_token", ""},
		{"special_chars", "token~!@#$%^&*()_+{}[]|:;<>,.?"},
		{"long_token", strings.Repeat("a", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashed := jwtmanager.HashToken(tt.token)

			if tt.token != "" {
				require.NotEmpty(t, hashed)
			}

			h := sha256.New()
			h.Write([]byte(tt.token))
			expected := base64.StdEncoding.EncodeToString(h.Sum(nil))

			require.Equal(t, expected, hashed)
		})
	}
}

func TestTokenClaimsHelpers(t *testing.T) {
	tests := []struct {
		name           string
		tokenType      string
		isAccessToken  bool
		isRefreshToken bool
	}{
		{"access_token", "access_token", true, false},
		{"refresh_token", "refresh_token", false, true},
		{"invalid_type", "invalid_type", false, false},
		{"empty_type", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &jwtmanager.TokenClaims{TokenType: tt.tokenType}

			require.Equal(t, tt.isAccessToken, claims.IsAccessToken())
			require.Equal(t, tt.isRefreshToken, claims.IsRefreshToken())
		})
	}
}

func TestJWTManager_ConcurrentUsage(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)
	userID := uuid.New()

	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			token, err := manager.GenerateAccessToken(userID, "user")
			if err != nil {
				results <- err
				return
			}

			_, err = manager.ParseToken(token)
			results <- err
		}()
	}

	for i := 0; i < 10; i++ {
		err := <-results
		require.NoError(t, err)
	}
}

func TestJWTManager_VeryShortTTL(t *testing.T) {
	manager := setupManager("secret", time.Nanosecond, time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "user")
	require.NoError(t, err)

	time.Sleep(time.Microsecond)

	_, err = manager.ParseToken(token)
	require.Error(t, err)
	require.True(t, errors.Is(err, jwt.ErrTokenExpired))
}

func TestJWTManager_RefreshTokenHasEmptyRole(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)
	userID := uuid.New()

	refreshToken, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	claims, err := manager.ParseToken(refreshToken)
	require.NoError(t, err)
	require.Equal(t, "", claims.Role)
}

func TestJWTManager_ExtractUserIDFromRefreshToken(t *testing.T) {
	manager := setupManager("secret", time.Minute, time.Hour)
	userID := uuid.New()

	refreshToken, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	extractedID, err := manager.ExtractUserID(refreshToken)
	require.NoError(t, err)
	require.Equal(t, userID, extractedID)
}

func TestHashToken_EdgeCases(t *testing.T) {
	t.Run("very_long_token", func(t *testing.T) {
		longToken := strings.Repeat("abcd", 10000)
		hashed := jwtmanager.HashToken(longToken)
		require.NotEmpty(t, hashed)

		h := sha256.New()
		h.Write([]byte(longToken))
		expected := base64.StdEncoding.EncodeToString(h.Sum(nil))
		require.Equal(t, expected, hashed)
	})
}
