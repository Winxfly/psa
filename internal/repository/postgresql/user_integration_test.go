//go:build integration

// Интеграционные тесты для user репозитория.
// Каждый тест поднимает свой контейнер для полной изоляции.
package postgresql_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"psa/internal/config"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPathUser = "migrations"

func mustParsePortForUser(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForUser(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForUser(t, port),
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

func setupTestDBUser(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathUser)
	require.NoError(t, err)

	return createStorageForUser(t, pg.DSN, pg.Host, pg.Port)
}

func createUser(ctx context.Context, t *testing.T, storage *postgresql.Storage, email, password string, isAdmin bool) uuid.UUID {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	_, err = storage.Pool.Exec(ctx, `
		INSERT INTO users (id, email, hashed_password, is_admin, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, userID, email, string(hashedPassword), isAdmin)
	require.NoError(t, err)

	return userID
}

func cleanUsersTable(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func TestUserRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBUser(t)

	t.Run("GetUserByEmail_Success", func(t *testing.T) {
		cleanUsersTable(ctx, t, storage)

		// Создание тестового пользователя
		email := "test1@example.com"
		password := "password123"
		userID := createUser(ctx, t, storage, email, password, false)

		// Тест
		user, err := storage.GetUserByEmail(ctx, email)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, userID, user.ID)
		require.Equal(t, email, user.Email)
		require.False(t, user.IsAdmin)
		require.NotEmpty(t, user.HashedPassword)
		require.False(t, user.CreatedAt.IsZero())
	})

	t.Run("GetUserByEmail_UserNotFound", func(t *testing.T) {
		cleanUsersTable(ctx, t, storage)

		// Тест (пользователь не создан)
		user, err := storage.GetUserByEmail(ctx, "nonexistent1@example.com")

		// Assert
		require.NoError(t, err)
		require.Nil(t, user)
	})

	t.Run("GetUserByEmail_AdminUser", func(t *testing.T) {
		cleanUsersTable(ctx, t, storage)

		// Создание админа
		email := "admin1@example.com"
		password := "admin123"
		userID := createUser(ctx, t, storage, email, password, true)

		// Тест
		user, err := storage.GetUserByEmail(ctx, email)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, userID, user.ID)
		require.True(t, user.IsAdmin)
		require.NotEmpty(t, user.HashedPassword)
		require.False(t, user.CreatedAt.IsZero())
	})

	t.Run("GetUserByID_Success", func(t *testing.T) {
		cleanUsersTable(ctx, t, storage)

		// Создание тестового пользователя
		email := "test2@example.com"
		password := "password123"
		userID := createUser(ctx, t, storage, email, password, false)

		// Тест
		user, err := storage.GetUserByID(ctx, userID)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, userID, user.ID)
		require.Equal(t, email, user.Email)
		require.False(t, user.IsAdmin)
		require.NotEmpty(t, user.HashedPassword)
		require.False(t, user.CreatedAt.IsZero())
	})

	t.Run("GetUserByID_UserNotFound", func(t *testing.T) {
		cleanUsersTable(ctx, t, storage)

		// Тест (несуществующий ID)
		nonExistentID := uuid.New()
		user, err := storage.GetUserByID(ctx, nonExistentID)

		// Assert
		require.NoError(t, err)
		require.Nil(t, user)
	})
}
