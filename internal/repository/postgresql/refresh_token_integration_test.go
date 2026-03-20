//go:build integration

package postgresql_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

// setupTestDBRefresh поднимает отдельный контейнер для refresh_token тестов
// Это нужно для изоляции от других тестов (user, profession)
var (
	testStorageRT *postgresql.Storage
	testCtxRT     = context.Background()
)

func setupTestDBRefresh(t *testing.T) *postgresql.Storage {
	t.Helper()

	if testStorageRT != nil {
		return testStorageRT
	}

	pg, err := containers.StartPostgres(testCtxRT)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(testCtxRT)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	testStorageRT = createStorage(t, pg.DSN, pg.Host, pg.Port)
	return testStorageRT
}

func cleanRefreshTokensTable(t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(testCtxRT, `TRUNCATE refresh_tokens RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createRefreshToken(t *testing.T, storage *postgresql.Storage, userID uuid.UUID, hashedToken string, expiresAt time.Time) {
	t.Helper()

	_, err := storage.Pool.Exec(testCtxRT, `
		INSERT INTO refresh_tokens (user_id, hashed_token, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashedToken, expiresAt)
	require.NoError(t, err)
}

func createTestUserForRefreshToken(t *testing.T, storage *postgresql.Storage) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := userID.String() + "@example.com"
	_, err := storage.Pool.Exec(testCtxRT, `
		INSERT INTO users (id, email, hashed_password, is_admin, created_at)
		VALUES ($1, $2, $3, false, NOW())
	`, userID, email, "hashed_password")
	require.NoError(t, err)

	return userID
}

func TestRefreshTokenRepository(t *testing.T) {
	storage := setupTestDBRefresh(t)

	t.Run("CreateRefreshToken_Success", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)
		hashedToken := "hashed_test_token_123"

		token := &domain.RefreshToken{
			UserID:      userID,
			HashedToken: hashedToken,
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}

		// Тест
		err := storage.CreateRefreshToken(testCtxRT, token)

		// Assert
		require.NoError(t, err)

		// Проверяем что токен действительно создан
		result, err := storage.GetRefreshToken(testCtxRT, userID, hashedToken)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, userID, result.UserID)
		require.Equal(t, hashedToken, result.HashedToken)
		require.False(t, result.CreatedAt.IsZero())
		require.False(t, result.ExpiresAt.IsZero())
	})

	t.Run("GetRefreshToken_Success", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)
		hashedToken := "hashed_test_token_456"

		createRefreshToken(t, storage, userID, hashedToken, time.Now().Add(24*time.Hour))

		// Тест
		result, err := storage.GetRefreshToken(testCtxRT, userID, hashedToken)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, userID, result.UserID)
		require.Equal(t, hashedToken, result.HashedToken)
		require.False(t, result.CreatedAt.IsZero())
		require.False(t, result.ExpiresAt.IsZero())
	})

	t.Run("GetRefreshToken_NotFound", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)
		nonExistentToken := "non_existent_token"

		// Тест
		result, err := storage.GetRefreshToken(testCtxRT, userID, nonExistentToken)

		// Assert
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("DeleteRefreshToken_Success", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)
		hashedToken := "hashed_test_token_789"

		createRefreshToken(t, storage, userID, hashedToken, time.Now().Add(24*time.Hour))

		// Тест
		err := storage.DeleteRefreshToken(testCtxRT, userID, hashedToken)

		// Assert
		require.NoError(t, err)

		// Проверяем что токен удалён
		result, err := storage.GetRefreshToken(testCtxRT, userID, hashedToken)
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("DeleteRefreshToken_Idempotent", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)
		nonExistentToken := "non_existent_token"

		// Тест (удаление несуществующего токена не должно вызывать ошибку)
		err := storage.DeleteRefreshToken(testCtxRT, userID, nonExistentToken)

		// Assert
		require.NoError(t, err)
	})

	t.Run("DeleteExpiredRefreshTokens_Success", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)

		// Создаём просроченный токен
		expiresAt := time.Now().Add(-1 * time.Hour) // Уже истёк
		createRefreshToken(t, storage, userID, "expired_token", expiresAt)

		// Создаём валидный токен
		createRefreshToken(t, storage, userID, "valid_token", time.Now().Add(24*time.Hour))

		// Тест
		err := storage.DeleteExpiredRefreshTokens(testCtxRT)

		// Assert
		require.NoError(t, err)

		// Проверяем что просроченный токен удалён
		result, err := storage.GetRefreshToken(testCtxRT, userID, "expired_token")
		require.NoError(t, err)
		require.Nil(t, result)

		// Проверяем что валидный токен остался
		result, err = storage.GetRefreshToken(testCtxRT, userID, "valid_token")
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("DeleteExpiredRefreshTokens_NoExpired", func(t *testing.T) {
		cleanRefreshTokensTable(t, storage)

		userID := createTestUserForRefreshToken(t, storage)

		// Создаём только валидные токены
		createRefreshToken(t, storage, userID, "valid_token_1", time.Now().Add(24*time.Hour))
		createRefreshToken(t, storage, userID, "valid_token_2", time.Now().Add(48*time.Hour))

		// Тест
		err := storage.DeleteExpiredRefreshTokens(testCtxRT)

		// Assert
		require.NoError(t, err)

		// Проверяем что оба токена остались
		result1, err := storage.GetRefreshToken(testCtxRT, userID, "valid_token_1")
		require.NoError(t, err)
		require.NotNil(t, result1)

		result2, err := storage.GetRefreshToken(testCtxRT, userID, "valid_token_2")
		require.NoError(t, err)
		require.NotNil(t, result2)
	})
}
