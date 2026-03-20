//go:build integration

// Интеграционные тесты для scraping репозитория.
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
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPathScraping = "migrations"

func mustParsePortForScraping(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForScraping(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForScraping(t, port),
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

func setupTestDBScraping(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathScraping)
	require.NoError(t, err)

	return createStorageForScraping(t, pg.DSN, pg.Host, pg.Port)
}

func cleanScrapingTable(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE scraping RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createScrapingSession(ctx context.Context, t *testing.T, storage *postgresql.Storage, scrapedAt time.Time) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO scraping (id, scraped_at)
		VALUES ($1, $2)
	`, id, scrapedAt)
	require.NoError(t, err)

	return id
}

func TestScrapingRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBScraping(t)

	t.Run("CreateScrapingSession_Success", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Тест
		id, err := storage.CreateScrapingSession(ctx)

		// Assert
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, id)

		// Проверяем что сессия действительно создана
		scraping, err := storage.GetLatestScraping(ctx)
		require.NoError(t, err)
		require.Equal(t, id, scraping.ID)
		require.False(t, scraping.ScrapedAt.IsZero())
	})

	t.Run("CreateScrapingSession_MultipleSessions", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём несколько сессий
		id1, err := storage.CreateScrapingSession(ctx)
		require.NoError(t, err)

		id2, err := storage.CreateScrapingSession(ctx)
		require.NoError(t, err)

		id3, err := storage.CreateScrapingSession(ctx)
		require.NoError(t, err)

		// Assert - все ID разные
		require.NotEqual(t, id1, id2)
		require.NotEqual(t, id2, id3)
		require.NotEqual(t, id1, id3)

		// Проверяем что все сессии созданы
		scrapings, err := storage.GetAllScrapingDates(ctx)
		require.NoError(t, err)
		require.Len(t, scrapings, 3)
	})

	t.Run("GetLatestScraping_Success", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём несколько сессий с разным временем
		now := time.Now()
		createScrapingSession(ctx, t, storage, now.Add(-2*time.Hour))
		createScrapingSession(ctx, t, storage, now.Add(-1*time.Hour))
		latestID := createScrapingSession(ctx, t, storage, now)

		// Тест
		latest, err := storage.GetLatestScraping(ctx)

		// Assert
		require.NoError(t, err)
		require.Equal(t, latestID, latest.ID)
		require.WithinDuration(t, now, latest.ScrapedAt, time.Second)
	})

	t.Run("GetLatestScraping_Empty", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Тест (нет сессий)
		latest, err := storage.GetLatestScraping(ctx)

		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "GetLatestScraping")
		require.Empty(t, latest)
	})

	t.Run("GetLatestScraping_SingleSession", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём одну сессию
		sessionID := createScrapingSession(ctx, t, storage, time.Now())

		// Тест
		latest, err := storage.GetLatestScraping(ctx)

		// Assert
		require.NoError(t, err)
		require.Equal(t, sessionID, latest.ID)
		require.False(t, latest.ScrapedAt.IsZero())
	})

	t.Run("GetAllScrapingDates_Success", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём несколько сессий
		now := time.Now()
		id1 := createScrapingSession(ctx, t, storage, now.Add(-3*time.Hour))
		id2 := createScrapingSession(ctx, t, storage, now.Add(-2*time.Hour))
		id3 := createScrapingSession(ctx, t, storage, now.Add(-1*time.Hour))
		id4 := createScrapingSession(ctx, t, storage, now)

		// Тест
		scrapings, err := storage.GetAllScrapingDates(ctx)

		// Assert
		require.NoError(t, err)
		require.Len(t, scrapings, 4)

		// Проверяем порядок (ORDER BY scraped_at DESC)
		require.Equal(t, id4, scrapings[0].ID)
		require.Equal(t, id3, scrapings[1].ID)
		require.Equal(t, id2, scrapings[2].ID)
		require.Equal(t, id1, scrapings[3].ID)

		// Проверяем что время не нулевое
		for _, s := range scrapings {
			require.False(t, s.ScrapedAt.IsZero())
		}
	})

	t.Run("GetAllScrapingDates_Empty", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Тест (нет сессий)
		scrapings, err := storage.GetAllScrapingDates(ctx)

		// Assert
		require.NoError(t, err)
		require.Empty(t, scrapings)
	})

	t.Run("GetAllScrapingDates_SingleSession", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём одну сессию
		sessionID := createScrapingSession(ctx, t, storage, time.Now())

		// Тест
		scrapings, err := storage.GetAllScrapingDates(ctx)

		// Assert
		require.NoError(t, err)
		require.Len(t, scrapings, 1)
		require.Equal(t, sessionID, scrapings[0].ID)
		require.False(t, scrapings[0].ScrapedAt.IsZero())
	})

	t.Run("ExistsScrapingSessionInCurrMonth_True", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём сессию в текущем месяце
		createScrapingSession(ctx, t, storage, time.Now())

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(ctx)

		// Assert
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_False", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Тест (нет сессий)
		exists, err := storage.ExistsScrapingSessionInCurrMonth(ctx)

		// Assert
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_PreviousMonth", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём сессию в предыдущем месяце
		now := time.Now()
		lastMonth := now.AddDate(0, -1, 0)
		createScrapingSession(ctx, t, storage, lastMonth)

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(ctx)

		// Assert
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_MultipleSessions", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём несколько сессий в текущем месяце
		now := time.Now()
		createScrapingSession(ctx, t, storage, now)
		createScrapingSession(ctx, t, storage, now.Add(-1*time.Hour))
		createScrapingSession(ctx, t, storage, now.Add(-24*time.Hour))

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(ctx)

		// Assert
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_MixedSessions", func(t *testing.T) {
		cleanScrapingTable(ctx, t, storage)

		// Создаём сессии в разных месяцах
		now := time.Now()
		createScrapingSession(ctx, t, storage, now.AddDate(0, -2, 0)) // 2 месяца назад
		createScrapingSession(ctx, t, storage, now.AddDate(0, -1, 0)) // 1 месяц назад
		createScrapingSession(ctx, t, storage, now)                   // текущий месяц

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(ctx)

		// Assert
		require.NoError(t, err)
		require.True(t, exists)
	})
}
