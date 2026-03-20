//go:build integration

// Интеграционные тесты для refresh_token репозитория.
// Каждый тест поднимает свой контейнер для полной изоляции.
package postgresql_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"psa/internal/config"
	"psa/internal/domain"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPathRefresh = "migrations"

func mustParsePortForRefresh(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForRefresh(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForRefresh(t, port),
		Database: "test",
		SSLMode:  "disable",
	}

	storage, err := postgresql.New(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		storage.Close()
	})

	return storage
}

func setupTestDBRefresh(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathRefresh)
	require.NoError(t, err)

	return createStorageForRefresh(t, pg.DSN, pg.Host, pg.Port)
}

func cleanRefreshTokensTable(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE refresh_tokens RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createRefreshToken(ctx context.Context, t *testing.T, storage *postgresql.Storage, userID uuid.UUID, hashedToken string, expiresAt time.Time) {
	t.Helper()

	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, hashed_token, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashedToken, expiresAt)
	require.NoError(t, err)
}

func createTestUserForRefreshToken(ctx context.Context, t *testing.T, storage *postgresql.Storage) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := userID.String() + "@example.com"
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO users (id, email, hashed_password, is_admin, created_at)
		VALUES ($1, $2, $3, false, NOW())
	`, userID, email, "hashed_password")
	require.NoError(t, err)

	return userID
}

func TestRefreshTokenRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBRefresh(t)

	t.Run("CreateRefreshToken_Success", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)
		hashedToken := "hashed_test_token_123"

		token := &domain.RefreshToken{
			UserID:      userID,
			HashedToken: hashedToken,
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}

		// Тест
		err := storage.CreateRefreshToken(ctx, token)

		// Assert
		require.NoError(t, err)

		// Проверяем что токен действительно создан
		result, err := storage.GetRefreshToken(ctx, userID, hashedToken)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, userID, result.UserID)
		require.Equal(t, hashedToken, result.HashedToken)
		require.False(t, result.CreatedAt.IsZero())
		require.False(t, result.ExpiresAt.IsZero())
	})

	t.Run("GetRefreshToken_Success", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)
		hashedToken := "hashed_test_token_456"

		createRefreshToken(ctx, t, storage, userID, hashedToken, time.Now().Add(24*time.Hour))

		// Тест
		result, err := storage.GetRefreshToken(ctx, userID, hashedToken)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, userID, result.UserID)
		require.Equal(t, hashedToken, result.HashedToken)
		require.False(t, result.CreatedAt.IsZero())
		require.False(t, result.ExpiresAt.IsZero())
	})

	t.Run("GetRefreshToken_NotFound", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)
		nonExistentToken := "non_existent_token"

		// Тест
		result, err := storage.GetRefreshToken(ctx, userID, nonExistentToken)

		// Assert
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("DeleteRefreshToken_Success", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)
		hashedToken := "hashed_test_token_789"

		createRefreshToken(ctx, t, storage, userID, hashedToken, time.Now().Add(24*time.Hour))

		// Тест
		err := storage.DeleteRefreshToken(ctx, userID, hashedToken)

		// Assert
		require.NoError(t, err)

		// Проверяем что токен удалён
		result, err := storage.GetRefreshToken(ctx, userID, hashedToken)
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("DeleteRefreshToken_Idempotent", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)
		nonExistentToken := "non_existent_token"

		// Тест (удаление несуществующего токена не должно вызывать ошибку)
		err := storage.DeleteRefreshToken(ctx, userID, nonExistentToken)

		// Assert
		require.NoError(t, err)
	})

	t.Run("DeleteExpiredRefreshTokens_Success", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)

		// Создаём просроченный токен
		expiresAt := time.Now().Add(-1 * time.Hour) // Уже истёк
		createRefreshToken(ctx, t, storage, userID, "expired_token", expiresAt)

		// Создаём валидный токен
		createRefreshToken(ctx, t, storage, userID, "valid_token", time.Now().Add(24*time.Hour))

		// Тест
		err := storage.DeleteExpiredRefreshTokens(ctx)

		// Assert
		require.NoError(t, err)

		// Проверяем что просроченный токен удалён
		result, err := storage.GetRefreshToken(ctx, userID, "expired_token")
		require.NoError(t, err)
		require.Nil(t, result)

		// Проверяем что валидный токен остался
		result, err = storage.GetRefreshToken(ctx, userID, "valid_token")
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("DeleteExpiredRefreshTokens_NoExpired", func(t *testing.T) {
		cleanRefreshTokensTable(ctx, t, storage)

		userID := createTestUserForRefreshToken(ctx, t, storage)

		// Создаём только валидные токены
		createRefreshToken(ctx, t, storage, userID, "valid_token_1", time.Now().Add(24*time.Hour))
		createRefreshToken(ctx, t, storage, userID, "valid_token_2", time.Now().Add(48*time.Hour))

		// Тест
		err := storage.DeleteExpiredRefreshTokens(ctx)

		// Assert
		require.NoError(t, err)

		// Проверяем что оба токена остались
		result1, err := storage.GetRefreshToken(ctx, userID, "valid_token_1")
		require.NoError(t, err)
		require.NotNil(t, result1)

		result2, err := storage.GetRefreshToken(ctx, userID, "valid_token_2")
		require.NoError(t, err)
		require.NotNil(t, result2)
	})
}
