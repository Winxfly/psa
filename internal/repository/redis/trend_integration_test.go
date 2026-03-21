//go:build integration

// Интеграционные тесты для redis trend репозитория.
// Каждый тест поднимает свой контейнер для полной изоляции.
package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"psa/internal/config"
	"psa/internal/domain"
	"psa/tests/containers"
)

// clientTestTrend returns the underlying redis client for testing purposes.
// This method is only available in integration tests.
func (c *Cache) clientTestTrend() *redis.Client {
	return c.client
}

func createCacheForTrend(t *testing.T, addr string) *Cache {
	t.Helper()

	cfg := config.Redis{
		Addr:       addr,
		Password:   "",
		DB:         0,
		DefaultTTL: 24 * time.Hour,
	}

	cache, err := New(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		cache.Close()
	})

	return cache
}

func setupTestRedisTrend(t *testing.T) *Cache {
	t.Helper()

	ctx := context.Background()
	redisContainer, err := containers.StartRedis(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = redisContainer.Container.Terminate(ctx)
	})

	return createCacheForTrend(t, redisContainer.Addr)
}

func cleanTrendCache(ctx context.Context, t *testing.T, cache *Cache, professionID uuid.UUID) {
	t.Helper()
	key := fmt.Sprintf(ProfessionTrendKeyPrefix, professionID.String())
	err := cache.clientTestTrend().Del(ctx, key).Err()
	require.NoError(t, err)
}

func createProfessionTrend(professionID uuid.UUID, name string) *domain.ProfessionTrend {
	baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	return &domain.ProfessionTrend{
		ProfessionID:   professionID,
		ProfessionName: name,
		Data: []domain.StatDailyPoint{
			{Date: baseDate.AddDate(0, 0, -2), VacancyCount: 100},
			{Date: baseDate.AddDate(0, 0, -1), VacancyCount: 120},
			{Date: baseDate, VacancyCount: 150},
		},
	}
}

func TestTrendCache(t *testing.T) {
	ctx := context.Background()
	cache := setupTestRedisTrend(t)

	t.Run("SaveProfessionTrend_Success", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		trend := createProfessionTrend(professionID, "Go Developer")

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Проверяем каждое поле детально
		require.Equal(t, professionID, result.ProfessionID)
		require.Equal(t, "Go Developer", result.ProfessionName)
		require.Len(t, result.Data, 3)

		// Проверяем данные тренда
		require.Equal(t, int32(100), result.Data[0].VacancyCount)
		require.Equal(t, int32(120), result.Data[1].VacancyCount)
		require.Equal(t, int32(150), result.Data[2].VacancyCount)
	})

	t.Run("SaveProfessionTrend_EmptyData", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		trend := &domain.ProfessionTrend{
			ProfessionID:   professionID,
			ProfessionName: "Empty Trend Developer",
			Data:           []domain.StatDailyPoint{},
		}

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные сохранились с пустым списком
		result, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, result.Data)
	})

	t.Run("SaveProfessionTrend_Overwrite", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		// Сохраняем первые данные
		trend1 := createProfessionTrend(professionID, "Go Developer v1")
		err := cache.SaveProfessionTrend(ctx, professionID, trend1)
		require.NoError(t, err)

		// Проверяем
		result1, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.Equal(t, "Go Developer v1", result1.ProfessionName)

		// Перезаписываем новыми данными
		trend2 := createProfessionTrend(professionID, "Go Developer v2")
		trend2.Data = append(trend2.Data, domain.StatDailyPoint{
			Date:         time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			VacancyCount: 200,
		})

		err = cache.SaveProfessionTrend(ctx, professionID, trend2)
		require.NoError(t, err)

		// Проверяем что данные перезаписались
		result2, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.Equal(t, "Go Developer v2", result2.ProfessionName)
		require.Len(t, result2.Data, 4)
	})

	t.Run("GetProfessionTrend_NotFound", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		// Тест (ключ не существует)
		result, err := cache.GetProfessionTrend(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("SaveProfessionTrend_TTL", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		trend := createProfessionTrend(professionID, "TTL Test Developer")

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)
		require.NoError(t, err)

		// Проверяем TTL ключа
		key := fmt.Sprintf(ProfessionTrendKeyPrefix, professionID.String())
		ttl, err := cache.clientTestTrend().TTL(ctx, key).Result()
		require.NoError(t, err)

		// TTL должен быть установлен (~4h = ttl/6 от 24h)
		// Используем допуск 5 секунд для стабильности
		expectedTTL := 4 * time.Hour
		require.GreaterOrEqual(t, ttl, expectedTTL-5*time.Second)
		require.LessOrEqual(t, ttl, expectedTTL)
	})

	t.Run("SaveProfessionTrend_Expiration", func(t *testing.T) {
		// Тест на реальное истечение TTL
		// TTL = DefaultTTL/6 = 6s/6 = 1 секунда
		// Sleep(2s) гарантирует истечение TTL с запасом

		cfg := config.Redis{
			Addr:       cache.clientTestTrend().Options().Addr,
			Password:   "",
			DB:         0,
			DefaultTTL: 6 * time.Second, // TTL/6 = 1 секунда
		}

		shortTTLCache, err := New(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			shortTTLCache.Close()
		})

		professionID := uuid.New()
		trend := createProfessionTrend(professionID, "Expiring Developer")

		// Сохраняем
		err = shortTTLCache.SaveProfessionTrend(ctx, professionID, trend)
		require.NoError(t, err)

		// Проверяем что данные есть
		result1, err := shortTTLCache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result1)

		// Ждем истечения TTL
		time.Sleep(2 * time.Second)

		// Проверяем что данные исчезли
		result2, err := shortTTLCache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.Nil(t, result2)
	})

	t.Run("GetProfessionTrend_InvalidJSON", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		// Записываем невалидный JSON напрямую в Redis
		key := fmt.Sprintf(ProfessionTrendKeyPrefix, professionID.String())
		err := cache.clientTestTrend().Set(ctx, key, "invalid-json", time.Hour).Err()
		require.NoError(t, err)

		// Тест
		result, err := cache.GetProfessionTrend(ctx, professionID)

		// Assert - должна быть ошибка парсинга
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("SaveProfessionTrend_SingleDataPoint", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		trend := &domain.ProfessionTrend{
			ProfessionID:   professionID,
			ProfessionName: "Single Point Developer",
			Data: []domain.StatDailyPoint{
				{Date: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), VacancyCount: 50},
			},
		}

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Data, 1)
		require.Equal(t, int32(50), result.Data[0].VacancyCount)
	})

	t.Run("SaveProfessionTrend_ManyDataPoints", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		// Создаем много точек данных
		dataPoints := make([]domain.StatDailyPoint, 100)
		baseDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 100; i++ {
			dataPoints[i] = domain.StatDailyPoint{
				Date:         baseDate.AddDate(0, 0, -100+i),
				VacancyCount: int32(i * 10),
			}
		}

		trend := &domain.ProfessionTrend{
			ProfessionID:   professionID,
			ProfessionName: "Many Data Points Developer",
			Data:           dataPoints,
		}

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Data, 100)

		// Выборочно проверяем несколько значений для подтверждения целостности
		require.Equal(t, int32(0), result.Data[0].VacancyCount)    // первая запись
		require.Equal(t, int32(490), result.Data[49].VacancyCount) // средняя запись
		require.Equal(t, int32(990), result.Data[99].VacancyCount) // последняя запись
	})

	t.Run("SaveProfessionTrend_SpecialCharacters", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		// Данные со специальными символами, UTF-8, эмодзи
		trend := &domain.ProfessionTrend{
			ProfessionID:   professionID,
			ProfessionName: "Разработчик 🐍 (Python) <Senior>",
			Data: []domain.StatDailyPoint{
				{Date: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), VacancyCount: 100},
				{Date: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), VacancyCount: 120},
			},
		}

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные сохранились корректно
		result, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "Разработчик 🐍 (Python) <Senior>", result.ProfessionName)
		require.Len(t, result.Data, 2)
	})

	t.Run("SaveAndGetProfessionTrend_VerifyFields", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID)
		})

		name := "Test Profession Name"
		baseDate := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
		dataPoints := []domain.StatDailyPoint{
			{Date: baseDate.AddDate(0, 0, -2), VacancyCount: 80},
			{Date: baseDate.AddDate(0, 0, -1), VacancyCount: 90},
			{Date: baseDate, VacancyCount: 100},
		}

		trend := &domain.ProfessionTrend{
			ProfessionID:   professionID,
			ProfessionName: name,
			Data:           dataPoints,
		}

		// Тест
		err := cache.SaveProfessionTrend(ctx, professionID, trend)
		require.NoError(t, err)

		// Проверяем все поля
		result, err := cache.GetProfessionTrend(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Equal(t, professionID, result.ProfessionID)
		require.Equal(t, name, result.ProfessionName)
		require.Len(t, result.Data, 3)

		// Проверяем каждую точку данных
		for i, point := range result.Data {
			require.Equal(t, dataPoints[i].Date, point.Date)
			require.Equal(t, dataPoints[i].VacancyCount, point.VacancyCount)
		}
	})

	t.Run("SaveProfessionTrend_MultipleProfessions", func(t *testing.T) {
		professionID1 := uuid.New()
		professionID2 := uuid.New()
		t.Cleanup(func() {
			cleanTrendCache(ctx, t, cache, professionID1)
			cleanTrendCache(ctx, t, cache, professionID2)
		})

		// Сохраняем данные для двух профессий
		trend1 := createProfessionTrend(professionID1, "Go Developer")
		trend2 := createProfessionTrend(professionID2, "Python Developer")
		trend2.Data = append(trend2.Data, domain.StatDailyPoint{
			Date:         time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			VacancyCount: 200,
		})

		err := cache.SaveProfessionTrend(ctx, professionID1, trend1)
		require.NoError(t, err)

		err = cache.SaveProfessionTrend(ctx, professionID2, trend2)
		require.NoError(t, err)

		// Проверяем что данные не пересеклись
		result1, err := cache.GetProfessionTrend(ctx, professionID1)
		require.NoError(t, err)
		require.Equal(t, "Go Developer", result1.ProfessionName)
		require.Len(t, result1.Data, 3)

		result2, err := cache.GetProfessionTrend(ctx, professionID2)
		require.NoError(t, err)
		require.Equal(t, "Python Developer", result2.ProfessionName)
		require.Len(t, result2.Data, 4)
	})
}
