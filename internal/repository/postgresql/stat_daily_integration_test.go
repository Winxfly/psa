//go:build integration

// Интеграционные тесты для stat_daily репозитория.
// Каждый тест поднимает свой контейнер для полной изоляции.
package postgresql_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"psa/internal/config"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPathStatDaily = "migrations"

func mustParsePortForStatDaily(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForStatDaily(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForStatDaily(t, port),
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

func setupTestDBStatDaily(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathStatDaily)
	require.NoError(t, err)

	return createStorageForStatDaily(t, pg.DSN, pg.Host, pg.Port)
}

func cleanStatDailyTable(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE stat_daily RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func cleanStatDailyAndRelatedTables(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE stat_daily, profession RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createProfessionForStatDaily(ctx context.Context, t *testing.T, storage *postgresql.Storage, name, vacancyQuery string, isActive bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO profession (id, name, vacancy_query, is_active)
		VALUES ($1, $2, $3, $4)
	`, id, name, vacancyQuery, isActive)
	require.NoError(t, err)

	return id
}

func TestStatDailyRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBStatDaily(t)

	t.Run("SaveStatDaily_Success", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer SaveStatDaily_Success", "go developer", true)
		scrapedAt := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
		vacancyCount := 150

		// Тест
		err := storage.SaveStatDaily(ctx, professionID, vacancyCount, scrapedAt)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)
		require.NoError(t, err)
		require.Len(t, points, 1)
		require.Equal(t, int32(vacancyCount), points[0].VacancyCount)
		require.True(t, scrapedAt.Equal(points[0].Date), "ожидалось %v, получено %v", scrapedAt, points[0].Date)
	})

	t.Run("SaveStatDaily_MultipleStats", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Python Developer SaveStatDaily_MultipleStats", "python developer", true)

		// Используем фиксированные даты для детерминированности
		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		dates := []time.Time{
			baseDate.AddDate(0, 0, -3),
			baseDate.AddDate(0, 0, -2),
			baseDate.AddDate(0, 0, -1),
			baseDate,
		}
		counts := []int{100, 110, 120, 130}

		// Тест - сохраняем несколько записей
		for i, date := range dates {
			err := storage.SaveStatDaily(ctx, professionID, counts[i], date)
			require.NoError(t, err)
		}

		// Проверяем что данные действительно сохранились
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)
		require.NoError(t, err)
		require.Len(t, points, 4)

		// Сортируем для детерминированной проверки (не доверяем ORDER BY)
		sort.Slice(points, func(i, j int) bool {
			return points[i].Date.Before(points[j].Date)
		})

		for i, point := range points {
			require.Equal(t, int32(counts[i]), point.VacancyCount)
			require.True(t, dates[i].Equal(point.Date), "ожидалось %v, получено %v", dates[i], point.Date)
		}
	})

	t.Run("SaveStatDaily_InvalidProfession", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		fakeProfessionID := uuid.New()
		scrapedAt := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)

		// Тест - нарушение FK (профессия не существует)
		err := storage.SaveStatDaily(ctx, fakeProfessionID, 100, scrapedAt)

		// Assert
		require.Error(t, err)
	})

	t.Run("SaveStatDaily_ZeroVacancyCount", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer SaveStatDaily_ZeroVacancyCount", "go developer 1", true)
		scrapedAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		// Тест - сохраняем с нулевым количеством вакансий
		err := storage.SaveStatDaily(ctx, professionID, 0, scrapedAt)

		// Assert - проверяем что данные действительно сохранились
		require.NoError(t, err)
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)
		require.NoError(t, err)
		require.Len(t, points, 1)
		require.Equal(t, int32(0), points[0].VacancyCount)
		require.True(t, scrapedAt.Equal(points[0].Date), "ожидалось %v, получено %v", scrapedAt, points[0].Date)
	})

	t.Run("SaveStatDaily_LargeVacancyCount", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer SaveStatDaily_LargeVacancyCount", "go developer 2", true)
		scrapedAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		// Тест - сохраняем с большим количеством вакансий
		err := storage.SaveStatDaily(ctx, professionID, 1000000, scrapedAt)

		// Assert - проверяем что данные действительно сохранились
		require.NoError(t, err)
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)
		require.NoError(t, err)
		require.Len(t, points, 1)
		require.Equal(t, int32(1000000), points[0].VacancyCount)
		require.True(t, scrapedAt.Equal(points[0].Date), "ожидалось %v, получено %v", scrapedAt, points[0].Date)
	})

	t.Run("GetStatDailyByProfessionID_Success", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionID_Success", "go developer 3", true)

		// Используем фиксированные даты для детерминированности
		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		dates := []time.Time{
			baseDate.AddDate(0, 0, -3),
			baseDate.AddDate(0, 0, -2),
			baseDate.AddDate(0, 0, -1),
		}
		counts := []int{100, 150, 200}

		// Сохраняем несколько записей
		for i, date := range dates {
			err := storage.SaveStatDaily(ctx, professionID, counts[i], date)
			require.NoError(t, err)
		}

		// Тест
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Len(t, points, 3)

		// Сортируем для детерминированной проверки (не доверяем ORDER BY)
		sort.Slice(points, func(i, j int) bool {
			return points[i].Date.Before(points[j].Date)
		})

		// Проверяем порядок (по дате) и значения
		for i, point := range points {
			require.Equal(t, int32(counts[i]), point.VacancyCount)
			require.True(t, dates[i].Equal(point.Date), "ожидалось %v, получено %v", dates[i], point.Date)
		}
	})

	t.Run("GetStatDailyByProfessionID_SingleStat", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionID_SingleStat", "go developer 4", true)

		scrapedAt := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
		err := storage.SaveStatDaily(ctx, professionID, 250, scrapedAt)
		require.NoError(t, err)

		// Тест
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Len(t, points, 1)
		require.Equal(t, int32(250), points[0].VacancyCount)
		require.True(t, scrapedAt.Equal(points[0].Date), "ожидалось %v, получено %v", scrapedAt, points[0].Date)
	})

	t.Run("GetStatDailyByProfessionID_Empty", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionID_Empty", "go developer 5", true)

		// Тест (нет записей)
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, points)
	})

	t.Run("GetStatDailyByProfessionID_MultipleEntriesPerDay", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionID_MultipleEntriesPerDay", "go developer 6", true)

		// Используем фиксированную дату для детерминированности
		today := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		// Сохраняем несколько записей за один день
		err := storage.SaveStatDaily(ctx, professionID, 100, today.Add(8*time.Hour))
		require.NoError(t, err)

		err = storage.SaveStatDaily(ctx, professionID, 150, today.Add(12*time.Hour))
		require.NoError(t, err)

		err = storage.SaveStatDaily(ctx, professionID, 200, today.Add(16*time.Hour))
		require.NoError(t, err)

		// Тест - DISTINCT ON должен вернуть одну запись за день (последнюю)
		points, err := storage.GetStatDailyByProfessionID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Len(t, points, 1)
		// Должна вернуться последняя запись за день (с максимальным временем)
		require.Equal(t, int32(200), points[0].VacancyCount)
		// Проверяем что вернулась именно последняя запись по времени
		expectedTime := today.Add(16 * time.Hour)
		require.True(t, expectedTime.Equal(points[0].Date), "ожидалось %v, получено %v", expectedTime, points[0].Date)
	})

	t.Run("GetStatDailyByProfessionIDs_Success", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID1 := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionIDs_Success_1", "go developer 7", true)
		professionID2 := createProfessionForStatDaily(ctx, t, storage, "Python Developer GetStatDailyByProfessionIDs_Success_2", "python developer 7", true)

		// Используем фиксированные даты для детерминированности
		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		// Сохраняем записи для первой профессии
		err := storage.SaveStatDaily(ctx, professionID1, 100, baseDate.AddDate(0, 0, -1))
		require.NoError(t, err)

		err = storage.SaveStatDaily(ctx, professionID1, 150, baseDate)
		require.NoError(t, err)

		// Сохраняем записи для второй профессии
		err = storage.SaveStatDaily(ctx, professionID2, 200, baseDate.AddDate(0, 0, -1))
		require.NoError(t, err)

		err = storage.SaveStatDaily(ctx, professionID2, 250, baseDate)
		require.NoError(t, err)

		// Тест
		result, err := storage.GetStatDailyByProfessionIDs(ctx, []uuid.UUID{professionID1, professionID2})

		// Assert
		require.NoError(t, err)
		require.Len(t, result, 2)

		// Проверяем данные для первой профессии
		require.Contains(t, result, professionID1)
		require.Len(t, result[professionID1], 2)
		counts1 := []int32{100, 150}
		dates1 := []time.Time{baseDate.AddDate(0, 0, -1), baseDate}
		points1 := result[professionID1]
		// Сортируем для детерминированной проверки
		sort.Slice(points1, func(i, j int) bool {
			return points1[i].Date.Before(points1[j].Date)
		})
		for i, point := range points1 {
			require.Equal(t, counts1[i], point.VacancyCount)
			require.True(t, dates1[i].Equal(point.Date), "ожидалось %v, получено %v", dates1[i], point.Date)
		}

		// Проверяем данные для второй профессии
		require.Contains(t, result, professionID2)
		require.Len(t, result[professionID2], 2)
		counts2 := []int32{200, 250}
		dates2 := []time.Time{baseDate.AddDate(0, 0, -1), baseDate}
		points2 := result[professionID2]
		// Сортируем для детерминированной проверки
		sort.Slice(points2, func(i, j int) bool {
			return points2[i].Date.Before(points2[j].Date)
		})
		for i, point := range points2 {
			require.Equal(t, counts2[i], point.VacancyCount)
			require.True(t, dates2[i].Equal(point.Date), "ожидалось %v, получено %v", dates2[i], point.Date)
		}
	})

	t.Run("GetStatDailyByProfessionIDs_SingleProfession", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionIDs_SingleProfession", "go developer 8", true)

		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		err := storage.SaveStatDaily(ctx, professionID, 100, baseDate.AddDate(0, 0, -1))
		require.NoError(t, err)

		err = storage.SaveStatDaily(ctx, professionID, 150, baseDate)
		require.NoError(t, err)

		// Тест
		result, err := storage.GetStatDailyByProfessionIDs(ctx, []uuid.UUID{professionID})

		// Assert
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Contains(t, result, professionID)
		require.Len(t, result[professionID], 2)
	})

	t.Run("GetStatDailyByProfessionIDs_Empty", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionIDs_Empty", "go developer 9", true)

		// Тест (нет записей)
		result, err := storage.GetStatDailyByProfessionIDs(ctx, []uuid.UUID{professionID})

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetStatDailyByProfessionIDs_EmptyProfessionIDs", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		// Тест - пустой список professionIDs
		result, err := storage.GetStatDailyByProfessionIDs(ctx, []uuid.UUID{})

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetStatDailyByProfessionIDs_MultipleDays", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		professionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionIDs_MultipleDays", "go developer 10", true)

		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		dates := []time.Time{
			baseDate.AddDate(0, 0, -5),
			baseDate.AddDate(0, 0, -3),
			baseDate.AddDate(0, 0, -1),
		}
		counts := []int{100, 150, 200}

		// Сохраняем записи
		for i, date := range dates {
			err := storage.SaveStatDaily(ctx, professionID, counts[i], date)
			require.NoError(t, err)
		}

		// Тест
		result, err := storage.GetStatDailyByProfessionIDs(ctx, []uuid.UUID{professionID})

		// Assert
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Contains(t, result, professionID)
		require.Len(t, result[professionID], 3)

		// Сортируем для детерминированной проверки (не доверяем ORDER BY)
		points := result[professionID]
		sort.Slice(points, func(i, j int) bool {
			return points[i].Date.Before(points[j].Date)
		})
		for i, point := range points {
			require.Equal(t, int32(counts[i]), point.VacancyCount)
			require.True(t, dates[i].Equal(point.Date), "ожидалось %v, получено %v", dates[i], point.Date)
		}
	})

	t.Run("GetStatDailyByProfessionIDs_MixedExistingAndNonExisting", func(t *testing.T) {
		t.Cleanup(func() {
			cleanStatDailyAndRelatedTables(ctx, t, storage)
		})

		existingProfessionID := createProfessionForStatDaily(ctx, t, storage, "Go Developer GetStatDailyByProfessionIDs_MixedExistingAndNonExisting", "go developer 11", true)
		nonExistingProfessionID := uuid.New()

		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		err := storage.SaveStatDaily(ctx, existingProfessionID, 100, baseDate)
		require.NoError(t, err)

		// Тест - передаём существующий и несуществующий ID
		result, err := storage.GetStatDailyByProfessionIDs(ctx, []uuid.UUID{existingProfessionID, nonExistingProfessionID})

		// Assert - должны вернуться только данные для существующей профессии
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Contains(t, result, existingProfessionID)
		require.NotContains(t, result, nonExistingProfessionID)
		require.Len(t, result[existingProfessionID], 1)
		require.Equal(t, int32(100), result[existingProfessionID][0].VacancyCount)
		require.True(t, baseDate.Equal(result[existingProfessionID][0].Date), "ожидалось %v, получено %v", baseDate, result[existingProfessionID][0].Date)
	})
}
