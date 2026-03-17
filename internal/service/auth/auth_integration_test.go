//go:build integration

package auth_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"psa/internal/config"
	"psa/internal/repository/postgresql"
	"psa/internal/service/auth"
	"psa/pkg/jwtmanager"
	"psa/tests/containers"
)

const migrationsPath = "migrations"

// createTestUser создаёт тестового пользователя в БД.
// Возвращает ID пользователя для последующего использования в тестах.
func createTestUser(t *testing.T, db *postgresql.Storage, email, password string, isAdmin bool) uuid.UUID {
	t.Helper()

	ctx := context.Background()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, hashed_password, is_admin, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, userID, email, string(hashedPassword), isAdmin)
	require.NoError(t, err)

	return userID
}

// mustParsePort конвертирует строку порта в int.
func mustParsePort(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

// createAuthUC создаёт настроенный Auth сервис для тестов.
func createAuthUC(t *testing.T, db *postgresql.Storage) *auth.Auth {
	t.Helper()

	jwtManager := jwtmanager.NewJWT("test-secret-key-min-32-chars!", 15*time.Minute, 168*time.Hour, "test")
	jwtAdapter := auth.NewJWTAdapter(jwtManager)

	authUC := auth.New(
		db,
		db,
		jwtAdapter,
		15*time.Minute,
		168*time.Hour,
	)

	return authUC
}

func TestAuth_Signin_Success(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание тестового пользователя
	userID := createTestUser(t, db, "test@example.com", "password123", false)

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Тест
	tokens, err := authUC.Signin(ctx, "test@example.com", "password123")

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)
	require.NotEmpty(t, userID)
}

func TestAuth_Signin_InvalidCredentials(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание тестового пользователя
	createTestUser(t, db, "test@example.com", "password123", false)

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Тест
	tokens, err := authUC.Signin(ctx, "test@example.com", "wrong-password")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuth_Signin_UserNotFound(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание Auth сервиса (без создания пользователя)
	authUC := createAuthUC(t, db)

	// Тест
	tokens, err := authUC.Signin(ctx, "nonexistent@example.com", "password123")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuth_RefreshTokens_Success(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание тестового пользователя
	createTestUser(t, db, "test@example.com", "password123", false)

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Сначала signin для получения refresh token
	tokens, err := authUC.Signin(ctx, "test@example.com", "password123")
	require.NoError(t, err)

	// Тест
	newTokens, err := authUC.RefreshTokens(ctx, tokens.RefreshToken)

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, newTokens.AccessToken)
	require.NotEmpty(t, newTokens.RefreshToken)
	t.Logf("Old access token: %s", tokens.AccessToken)
	t.Logf("New access token: %s", newTokens.AccessToken)
	require.NotEqual(t, tokens.AccessToken, newTokens.AccessToken, "Access tokens should be different")
	require.NotEqual(t, tokens.RefreshToken, newTokens.RefreshToken, "Refresh tokens should be different")
}

func TestAuth_RefreshTokens_InvalidToken(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Тест с невалидным токеном
	tokens, err := authUC.RefreshTokens(ctx, "invalid-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, tokens)
}

func TestAuth_Logout_Success(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание тестового пользователя
	createTestUser(t, db, "test@example.com", "password123", false)

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Сначала signin
	tokens, err := authUC.Signin(ctx, "test@example.com", "password123")
	require.NoError(t, err)

	// Тест
	err = authUC.Logout(ctx, tokens.RefreshToken)

	// Assert
	require.NoError(t, err)

	// Попытка использовать тот же refresh token должна失败
	_, err = authUC.RefreshTokens(ctx, tokens.RefreshToken)
	require.Error(t, err)
}

func TestAuth_ValidateToken_Success(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание тестового пользователя
	createTestUser(t, db, "test@example.com", "password123", false)

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Signin для получения токена
	tokens, err := authUC.Signin(ctx, "test@example.com", "password123")
	require.NoError(t, err)

	// Тест
	claims, err := authUC.ValidateToken(ctx, tokens.AccessToken)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, "user", claims.Role)
}

func TestAuth_ValidateToken_InvalidToken(t *testing.T) {
	ctx := context.Background()

	// Запуск PostgreSQL
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)
	defer pg.Container.Terminate(ctx)

	// Запуск миграций
	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	// Создание репозитория
	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     pg.Host,
		Port:     mustParsePort(t, pg.Port),
		Database: "test",
		SSLMode:  "disable",
	}

	db, err := postgresql.New(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создание Auth сервиса
	authUC := createAuthUC(t, db)

	// Тест с невалидным токеном
	claims, err := authUC.ValidateToken(ctx, "invalid-token")

	// Assert
	require.Error(t, err)
	require.Nil(t, claims)
}
