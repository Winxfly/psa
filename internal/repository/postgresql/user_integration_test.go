//go:build integration

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

func mustParsePort(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorage(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePort(t, port),
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

// setupTestDB поднимает БД один раз на все тесты в пакете
var (
	testStorageUser *postgresql.Storage
	testCtxUser     = context.Background()
)

func setupTestDBUser(t *testing.T) *postgresql.Storage {
	t.Helper()

	if testStorageUser != nil {
		return testStorageUser
	}

	pg, err := containers.StartPostgres(testCtxUser)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(testCtxUser)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	testStorageUser = createStorage(t, pg.DSN, pg.Host, pg.Port)
	return testStorageUser
}

func createUser(t *testing.T, storage *postgresql.Storage, email, password string, isAdmin bool) uuid.UUID {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	_, err = storage.Pool.Exec(testCtxUser, `
		INSERT INTO users (id, email, hashed_password, is_admin, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, userID, email, string(hashedPassword), isAdmin)
	require.NoError(t, err)

	return userID
}

func cleanUsersTable(t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(testCtxUser, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func TestUserRepository(t *testing.T) {
	storage := setupTestDBUser(t)

	t.Run("GetUserByEmail_Success", func(t *testing.T) {
		cleanUsersTable(t, storage)

		// Создание тестового пользователя
		email := "test1@example.com"
		password := "password123"
		userID := createUser(t, storage, email, password, false)

		// Тест
		user, err := storage.GetUserByEmail(testCtxUser, email)

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
		cleanUsersTable(t, storage)

		// Тест (пользователь не создан)
		user, err := storage.GetUserByEmail(testCtxUser, "nonexistent1@example.com")

		// Assert
		require.NoError(t, err)
		require.Nil(t, user)
	})

	t.Run("GetUserByEmail_AdminUser", func(t *testing.T) {
		cleanUsersTable(t, storage)

		// Создание админа
		email := "admin1@example.com"
		password := "admin123"
		userID := createUser(t, storage, email, password, true)

		// Тест
		user, err := storage.GetUserByEmail(testCtxUser, email)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, userID, user.ID)
		require.True(t, user.IsAdmin)
		require.NotEmpty(t, user.HashedPassword)
		require.False(t, user.CreatedAt.IsZero())
	})

	t.Run("GetUserByID_Success", func(t *testing.T) {
		cleanUsersTable(t, storage)

		// Создание тестового пользователя
		email := "test2@example.com"
		password := "password123"
		userID := createUser(t, storage, email, password, false)

		// Тест
		user, err := storage.GetUserByID(testCtxUser, userID)

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
		cleanUsersTable(t, storage)

		// Тест (несуществующий ID)
		nonExistentID := uuid.New()
		user, err := storage.GetUserByID(testCtxUser, nonExistentID)

		// Assert
		require.NoError(t, err)
		require.Nil(t, user)
	})
}
