//go:build integration

// Интеграционные тесты для stat репозитория.
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

const migrationsPathStat = "migrations"

func mustParsePortForStat(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForStat(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForStat(t, port),
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

func setupTestDBStat(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathStat)
	require.NoError(t, err)

	return createStorageForStat(t, pg.DSN, pg.Host, pg.Port)
}

func cleanStatTable(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE stat RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func cleanStatAndRelatedTables(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE stat, scraping, profession RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createProfessionForStat(ctx context.Context, t *testing.T, storage *postgresql.Storage, name, vacancyQuery string, isActive bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO profession (id, name, vacancy_query, is_active)
		VALUES ($1, $2, $3, $4)
	`, id, name, vacancyQuery, isActive)
	require.NoError(t, err)

	return id
}

func createScrapingSessionForStat(ctx context.Context, t *testing.T, storage *postgresql.Storage, scrapedAt time.Time) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO scraping (id, scraped_at)
		VALUES ($1, $2)
	`, id, scrapedAt)
	require.NoError(t, err)

	return id
}

func TestStatRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBStat(t)

	t.Run("SaveStat_Success", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer", "go developer", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		vacancyCount := 150

		// Тест
		err := storage.SaveStat(ctx, sessionID, professionID, vacancyCount)

		// Assert
		require.NoError(t, err)
	})

	t.Run("SaveStat_MultipleStats", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Python Developer", "python developer", true)
		sessionID1 := createScrapingSessionForStat(ctx, t, storage, time.Now().Add(-24*time.Hour))
		sessionID2 := createScrapingSessionForStat(ctx, t, storage, time.Now())

		// Тест - сохраняем несколько записей
		err := storage.SaveStat(ctx, sessionID1, professionID, 100)
		require.NoError(t, err)

		err = storage.SaveStat(ctx, sessionID2, professionID, 150)
		require.NoError(t, err)
	})

	t.Run("SaveStat_InvalidProfession", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())
		fakeProfessionID := uuid.New()

		// Тест - нарушение FK (профессия не существует)
		err := storage.SaveStat(ctx, sessionID, fakeProfessionID, 100)

		// Assert
		require.Error(t, err)
	})

	t.Run("SaveStat_InvalidSession", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Java Developer", "java developer", true)
		fakeSessionID := uuid.New()

		// Тест - нарушение FK (сессия не существует)
		err := storage.SaveStat(ctx, fakeSessionID, professionID, 100)

		// Assert
		require.Error(t, err)
	})

	t.Run("GetLatestStatByProfessionID_Success", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #1", "go developer 1", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		// Сохраняем запись
		err := storage.SaveStat(ctx, sessionID, professionID, 150)
		require.NoError(t, err)

		// Тест - получаем последнюю запись
		stat, err := storage.GetLatestStatByProfessionID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID, stat.ProfessionID)
		require.Equal(t, sessionID, stat.ScrapedAtID)
		require.Equal(t, int32(150), stat.VacancyCount)
	})

	t.Run("GetLatestStatByProfessionID_SingleStat", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #2", "go developer 2", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		err := storage.SaveStat(ctx, sessionID, professionID, 200)
		require.NoError(t, err)

		// Тест
		stat, err := storage.GetLatestStatByProfessionID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID, stat.ProfessionID)
		require.Equal(t, int32(200), stat.VacancyCount)
		require.Equal(t, sessionID, stat.ScrapedAtID)
	})

	t.Run("GetLatestStatByProfessionID_NotFound", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #3", "go developer 3", true)

		// Тест (нет записей для профессии)
		stat, err := storage.GetLatestStatByProfessionID(ctx, professionID)

		// Assert
		require.Error(t, err)
		require.Empty(t, stat)
	})

	t.Run("GetLatestStatByProfessionID_MultipleProfessions", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID1 := createProfessionForStat(ctx, t, storage, "Go Developer #4", "go developer 4", true)
		professionID2 := createProfessionForStat(ctx, t, storage, "Python Developer #4", "python developer 4", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		// Сохраняем записи для двух профессий
		err := storage.SaveStat(ctx, sessionID, professionID1, 100)
		require.NoError(t, err)

		err = storage.SaveStat(ctx, sessionID, professionID2, 200)
		require.NoError(t, err)

		// Тест - получаем последнюю запись для первой профессии
		stat, err := storage.GetLatestStatByProfessionID(ctx, professionID1)

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID1, stat.ProfessionID)
		require.Equal(t, int32(100), stat.VacancyCount)
	})

	t.Run("GetStatsByProfessionsAndDateRange_Success", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID1 := createProfessionForStat(ctx, t, storage, "Go Developer #5", "go developer 5", true)
		professionID2 := createProfessionForStat(ctx, t, storage, "Python Developer #5", "python developer 5", true)

		now := time.Now()
		sessionID1 := createScrapingSessionForStat(ctx, t, storage, now.Add(-48*time.Hour))
		sessionID2 := createScrapingSessionForStat(ctx, t, storage, now.Add(-24*time.Hour))
		sessionID3 := createScrapingSessionForStat(ctx, t, storage, now)

		// Сохраняем записи
		err := storage.SaveStat(ctx, sessionID1, professionID1, 100)
		require.NoError(t, err)

		err = storage.SaveStat(ctx, sessionID2, professionID1, 150)
		require.NoError(t, err)

		err = storage.SaveStat(ctx, sessionID3, professionID2, 200)
		require.NoError(t, err)

		// Тест - получаем записи за последние 3 дня с запасом
		startDate := now.Add(-72 * time.Hour).Format(time.RFC3339)
		endDate := now.Add(1 * time.Hour).Format(time.RFC3339)

		stats, err := storage.GetStatsByProfessionsAndDateRange(ctx, []uuid.UUID{professionID1, professionID2}, startDate, endDate)

		// Assert - ожидаем 3 записи (2 для professionID1 + 1 для professionID2)
		require.NoError(t, err)
		require.Len(t, stats, 3)

		// Проверяем что все записи относятся к нашим профессиям
		for _, stat := range stats {
			require.Contains(t, []uuid.UUID{professionID1, professionID2}, stat.ProfessionID)
		}
	})

	t.Run("GetStatsByProfessionsAndDateRange_SingleProfession", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #6", "go developer 6", true)

		now := time.Now()
		sessionID1 := createScrapingSessionForStat(ctx, t, storage, now.Add(-24*time.Hour))
		sessionID2 := createScrapingSessionForStat(ctx, t, storage, now)

		err := storage.SaveStat(ctx, sessionID1, professionID, 100)
		require.NoError(t, err)

		err = storage.SaveStat(ctx, sessionID2, professionID, 150)
		require.NoError(t, err)

		// Тест - расширяем диапазон чтобы точно попасть
		startDate := now.Add(-48 * time.Hour).Format(time.RFC3339)
		endDate := now.Add(1 * time.Hour).Format(time.RFC3339)

		stats, err := storage.GetStatsByProfessionsAndDateRange(ctx, []uuid.UUID{professionID}, startDate, endDate)

		// Assert
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(stats), 1)
		require.Equal(t, professionID, stats[0].ProfessionID)
	})

	t.Run("GetStatsByProfessionsAndDateRange_Empty", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #7", "go developer 7", true)

		// Тест (нет записей)
		startDate := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		endDate := time.Now().Format(time.RFC3339)

		stats, err := storage.GetStatsByProfessionsAndDateRange(ctx, []uuid.UUID{professionID}, startDate, endDate)

		// Assert
		require.NoError(t, err)
		require.Empty(t, stats)
	})

	t.Run("GetStatsByProfessionsAndDateRange_InvalidDateRange", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #8", "go developer 8", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		err := storage.SaveStat(ctx, sessionID, professionID, 100)
		require.NoError(t, err)

		// Тест - некорректный формат даты
		stats, err := storage.GetStatsByProfessionsAndDateRange(ctx, []uuid.UUID{professionID}, "invalid-date", "2026-01-01")

		// Assert
		require.Error(t, err)
		require.Empty(t, stats)
	})

	t.Run("GetStatsByProfessionsAndDateRange_EmptyProfessionIDs", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		now := time.Now()
		sessionID := createScrapingSessionForStat(ctx, t, storage, now)
		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #11", "go developer 11", true)

		err := storage.SaveStat(ctx, sessionID, professionID, 100)
		require.NoError(t, err)

		// Тест - пустой список professionIDs
		startDate := now.Add(-24 * time.Hour).Format(time.RFC3339)
		endDate := now.Add(1 * time.Hour).Format(time.RFC3339)

		stats, err := storage.GetStatsByProfessionsAndDateRange(ctx, []uuid.UUID{}, startDate, endDate)

		// Assert
		require.NoError(t, err)
		require.Empty(t, stats)
	})

	t.Run("SaveStat_ZeroVacancyCount", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #9", "go developer 9", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		// Тест - сохраняем с нулевым количеством вакансий
		err := storage.SaveStat(ctx, sessionID, professionID, 0)

		// Assert
		require.NoError(t, err)
	})

	t.Run("SaveStat_LargeVacancyCount", func(t *testing.T) {
		cleanStatAndRelatedTables(ctx, t, storage)

		professionID := createProfessionForStat(ctx, t, storage, "Go Developer #10", "go developer 10", true)
		sessionID := createScrapingSessionForStat(ctx, t, storage, time.Now())

		// Тест - сохраняем с большим количеством вакансий
		err := storage.SaveStat(ctx, sessionID, professionID, 1000000)

		// Assert
		require.NoError(t, err)
	})
}
