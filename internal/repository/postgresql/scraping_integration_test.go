//go:build integration

package postgresql_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

// setupTestDBScraping поднимает отдельный контейнер для scraping тестов
// Это нужно для изоляции от других тестов (user, profession, refresh_token)
var (
	testStorageScraping *postgresql.Storage
	testCtxScraping     = context.Background()
)

func setupTestDBScraping(t *testing.T) *postgresql.Storage {
	t.Helper()

	if testStorageScraping != nil {
		return testStorageScraping
	}

	pg, err := containers.StartPostgres(testCtxScraping)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(testCtxScraping)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	testStorageScraping = createStorage(t, pg.DSN, pg.Host, pg.Port)
	return testStorageScraping
}

func cleanScrapingTable(t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(testCtxScraping, `TRUNCATE scraping RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createScrapingSession(t *testing.T, storage *postgresql.Storage, scrapedAt time.Time) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(testCtxScraping, `
		INSERT INTO scraping (id, scraped_at)
		VALUES ($1, $2)
	`, id, scrapedAt)
	require.NoError(t, err)

	return id
}

func TestScrapingRepository(t *testing.T) {
	storage := setupTestDBScraping(t)

	t.Run("CreateScrapingSession_Success", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Тест
		id, err := storage.CreateScrapingSession(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, id)

		// Проверяем что сессия действительно создана
		scraping, err := storage.GetLatestScraping(testCtxScraping)
		require.NoError(t, err)
		require.Equal(t, id, scraping.ID)
		require.False(t, scraping.ScrapedAt.IsZero())
	})

	t.Run("CreateScrapingSession_MultipleSessions", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём несколько сессий
		id1, err := storage.CreateScrapingSession(testCtxScraping)
		require.NoError(t, err)

		id2, err := storage.CreateScrapingSession(testCtxScraping)
		require.NoError(t, err)

		id3, err := storage.CreateScrapingSession(testCtxScraping)
		require.NoError(t, err)

		// Assert - все ID разные
		require.NotEqual(t, id1, id2)
		require.NotEqual(t, id2, id3)
		require.NotEqual(t, id1, id3)

		// Проверяем что все сессии созданы
		scrapings, err := storage.GetAllScrapingDates(testCtxScraping)
		require.NoError(t, err)
		require.Len(t, scrapings, 3)
	})

	t.Run("GetLatestScraping_Success", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём несколько сессий с разным временем
		now := time.Now()
		createScrapingSession(t, storage, now.Add(-2*time.Hour))
		createScrapingSession(t, storage, now.Add(-1*time.Hour))
		latestID := createScrapingSession(t, storage, now)

		// Тест
		latest, err := storage.GetLatestScraping(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.Equal(t, latestID, latest.ID)
		require.WithinDuration(t, now, latest.ScrapedAt, time.Second)
	})

	t.Run("GetLatestScraping_Empty", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Тест (нет сессий)
		latest, err := storage.GetLatestScraping(testCtxScraping)

		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "GetLatestScraping")
		require.Empty(t, latest)
	})

	t.Run("GetLatestScraping_SingleSession", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём одну сессию
		sessionID := createScrapingSession(t, storage, time.Now())

		// Тест
		latest, err := storage.GetLatestScraping(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.Equal(t, sessionID, latest.ID)
		require.False(t, latest.ScrapedAt.IsZero())
	})

	t.Run("GetAllScrapingDates_Success", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём несколько сессий
		now := time.Now()
		id1 := createScrapingSession(t, storage, now.Add(-3*time.Hour))
		id2 := createScrapingSession(t, storage, now.Add(-2*time.Hour))
		id3 := createScrapingSession(t, storage, now.Add(-1*time.Hour))
		id4 := createScrapingSession(t, storage, now)

		// Тест
		scrapings, err := storage.GetAllScrapingDates(testCtxScraping)

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
		cleanScrapingTable(t, storage)

		// Тест (нет сессий)
		scrapings, err := storage.GetAllScrapingDates(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.Empty(t, scrapings)
	})

	t.Run("GetAllScrapingDates_SingleSession", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём одну сессию
		sessionID := createScrapingSession(t, storage, time.Now())

		// Тест
		scrapings, err := storage.GetAllScrapingDates(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.Len(t, scrapings, 1)
		require.Equal(t, sessionID, scrapings[0].ID)
		require.False(t, scrapings[0].ScrapedAt.IsZero())
	})

	t.Run("ExistsScrapingSessionInCurrMonth_True", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём сессию в текущем месяце
		createScrapingSession(t, storage, time.Now())

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_False", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Тест (нет сессий)
		exists, err := storage.ExistsScrapingSessionInCurrMonth(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_PreviousMonth", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём сессию в предыдущем месяце
		now := time.Now()
		lastMonth := now.AddDate(0, -1, 0)
		createScrapingSession(t, storage, lastMonth)

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_MultipleSessions", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём несколько сессий в текущем месяце
		now := time.Now()
		createScrapingSession(t, storage, now)
		createScrapingSession(t, storage, now.Add(-1*time.Hour))
		createScrapingSession(t, storage, now.Add(-24*time.Hour))

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("ExistsScrapingSessionInCurrMonth_MixedSessions", func(t *testing.T) {
		cleanScrapingTable(t, storage)

		// Создаём сессии в разных месяцах
		now := time.Now()
		createScrapingSession(t, storage, now.AddDate(0, -2, 0)) // 2 месяца назад
		createScrapingSession(t, storage, now.AddDate(0, -1, 0)) // 1 месяц назад
		createScrapingSession(t, storage, now)                   // текущий месяц

		// Тест
		exists, err := storage.ExistsScrapingSessionInCurrMonth(testCtxScraping)

		// Assert
		require.NoError(t, err)
		require.True(t, exists)
	})
}
