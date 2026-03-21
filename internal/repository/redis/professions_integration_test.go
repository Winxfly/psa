//go:build integration

// Интеграционные тесты для redis professions репозитория.
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

// clientTestProfessions returns the underlying redis client for testing purposes.
// This method is only available in integration tests.
func (c *Cache) clientTestProfessions() *redis.Client {
	return c.client
}

func createCacheForProfessions(t *testing.T, addr string) *Cache {
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

func setupTestRedisProfessions(t *testing.T) *Cache {
	t.Helper()

	ctx := context.Background()
	redisContainer, err := containers.StartRedis(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = redisContainer.Container.Terminate(ctx)
	})

	return createCacheForProfessions(t, redisContainer.Addr)
}

func cleanProfessionsCache(ctx context.Context, t *testing.T, cache *Cache) {
	t.Helper()
	err := cache.clientTestProfessions().Del(ctx, ProfessionListKey).Err()
	require.NoError(t, err)
}

func TestProfessionsCache(t *testing.T) {
	ctx := context.Background()
	cache := setupTestRedisProfessions(t)

	t.Run("SaveProfessionsList_Success", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		id1 := uuid.New()
		id2 := uuid.New()
		professions := []domain.ActiveProfession{
			{
				ID:           id1,
				Name:         "Go Developer",
				VacancyQuery: "go developer",
			},
			{
				ID:           id2,
				Name:         "Python Developer",
				VacancyQuery: "python developer",
			},
		}

		// Тест
		err := cache.SaveProfessionsList(ctx, professions)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result, 2)

		// Проверяем каждое поле детально
		require.ElementsMatch(t, professions, result)

		// Находим и проверяем конкретные значения
		profMap := make(map[uuid.UUID]domain.ActiveProfession)
		for _, p := range result {
			profMap[p.ID] = p
		}

		require.Equal(t, "Go Developer", profMap[id1].Name)
		require.Equal(t, "go developer", profMap[id1].VacancyQuery)
		require.Equal(t, "Python Developer", profMap[id2].Name)
		require.Equal(t, "python developer", profMap[id2].VacancyQuery)
	})

	t.Run("SaveProfessionsList_Empty", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		professions := []domain.ActiveProfession{}

		// Тест
		err := cache.SaveProfessionsList(ctx, professions)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные сохранились как пустой список
		result, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("SaveProfessionsList_Overwrite", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		// Сохраняем первый список
		professions1 := []domain.ActiveProfession{
			{
				ID:           uuid.New(),
				Name:         "Go Developer",
				VacancyQuery: "go developer",
			},
		}
		err := cache.SaveProfessionsList(ctx, professions1)
		require.NoError(t, err)

		// Проверяем
		result1, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result1, 1)

		// Перезаписываем новым списком
		professions2 := []domain.ActiveProfession{
			{
				ID:           uuid.New(),
				Name:         "Python Developer",
				VacancyQuery: "python developer",
			},
			{
				ID:           uuid.New(),
				Name:         "Java Developer",
				VacancyQuery: "java developer",
			},
			{
				ID:           uuid.New(),
				Name:         "C++ Developer",
				VacancyQuery: "c++ developer",
			},
		}
		err = cache.SaveProfessionsList(ctx, professions2)
		require.NoError(t, err)

		// Проверяем что данные перезаписались
		result2, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result2, 3)
		require.ElementsMatch(t, professions2, result2)
	})

	t.Run("GetProfessionsList_NotFound", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		// Тест (ключ не существует)
		result, err := cache.GetProfessionsList(ctx)

		// Assert
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("SaveProfessionsList_TTL", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		professions := []domain.ActiveProfession{
			{
				ID:           uuid.New(),
				Name:         "Test Profession",
				VacancyQuery: "test query",
			},
		}

		// Тест
		err := cache.SaveProfessionsList(ctx, professions)
		require.NoError(t, err)

		// Проверяем TTL ключа
		ttl, err := cache.clientTestProfessions().TTL(ctx, ProfessionListKey).Result()
		require.NoError(t, err)

		// TTL должен быть установлен (~12h = ttl/2 от 24h)
		// Используем диапазон с допуском 1 час для стабильности
		require.Greater(t, ttl, 11*time.Hour)
		require.LessOrEqual(t, ttl, 12*time.Hour)
	})

	t.Run("SaveProfessionsList_Expiration", func(t *testing.T) {
		// Тест на реальное истечение TTL
		// Используем очень маленький TTL для быстрого теста

		// Создаем кэш с TTL 1 секунда
		cfg := config.Redis{
			Addr:       cache.clientTestProfessions().Options().Addr,
			Password:   "",
			DB:         0,
			DefaultTTL: 2 * time.Second, // ttl/2 = 1 секунда
		}

		shortTTLCache, err := New(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			shortTTLCache.Close()
		})

		professions := []domain.ActiveProfession{
			{
				ID:           uuid.New(),
				Name:         "Expiring Profession",
				VacancyQuery: "expiring query",
			},
		}

		// Сохраняем
		err = shortTTLCache.SaveProfessionsList(ctx, professions)
		require.NoError(t, err)

		// Проверяем что данные есть
		result1, err := shortTTLCache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result1, 1)

		// Ждем истечения TTL
		time.Sleep(2 * time.Second)

		// Проверяем что данные исчезли
		result2, err := shortTTLCache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Nil(t, result2)
	})

	t.Run("GetProfessionsList_InvalidJSON", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		// Записываем невалидный JSON напрямую в Redis
		err := cache.clientTestProfessions().Set(ctx, ProfessionListKey, "invalid-json", time.Hour).Err()
		require.NoError(t, err)

		// Тест
		result, err := cache.GetProfessionsList(ctx)

		// Assert - должна быть ошибка парсинга
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("SaveProfessionsList_SingleProfession", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		profession := []domain.ActiveProfession{
			{
				ID:           uuid.New(),
				Name:         "Single Profession",
				VacancyQuery: "single profession query",
			},
		}

		// Тест
		err := cache.SaveProfessionsList(ctx, profession)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, profession[0].ID, result[0].ID)
		require.Equal(t, profession[0].Name, result[0].Name)
		require.Equal(t, profession[0].VacancyQuery, result[0].VacancyQuery)
	})

	t.Run("SaveProfessionsList_ManyProfessions", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		// Создаем большой список профессий
		professions := make([]domain.ActiveProfession, 100)
		for i := 0; i < 100; i++ {
			professions[i] = domain.ActiveProfession{
				ID:           uuid.New(),
				Name:         fmt.Sprintf("Profession_%d", i),
				VacancyQuery: fmt.Sprintf("query_%d", i),
			}
		}

		// Тест
		err := cache.SaveProfessionsList(ctx, professions)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result, 100)
		require.ElementsMatch(t, professions, result)
	})

	t.Run("SaveProfessionsList_SpecialCharacters", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		// Профессии со специальными символами, UTF-8, эмодзи
		professions := []domain.ActiveProfession{
			{
				ID:           uuid.New(),
				Name:         "Go Developer (Golang) <Senior>",
				VacancyQuery: "go AND (senior OR lead) -junior",
			},
			{
				ID:           uuid.New(),
				Name:         "Разработчик Python 🐍",
				VacancyQuery: "python разработчик программирование",
			},
			{
				ID:           uuid.New(),
				Name:         "Java Developer \"Middle\"",
				VacancyQuery: `java developer "spring" 'hibernate'`,
			},
			{
				ID:           uuid.New(),
				Name:         "C++ Developer\nMulti-line\tTab",
				VacancyQuery: "c++\tqt\tlinux",
			},
		}

		// Тест
		err := cache.SaveProfessionsList(ctx, professions)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные сохранились корректно
		result, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result, 4)
		require.ElementsMatch(t, professions, result)
	})

	t.Run("SaveAndGetProfessionsList_VerifyFields", func(t *testing.T) {
		t.Cleanup(func() {
			cleanProfessionsCache(ctx, t, cache)
		})

		id := uuid.New()
		name := "Test Profession Name"
		query := "test profession query"

		professions := []domain.ActiveProfession{
			{
				ID:           id,
				Name:         name,
				VacancyQuery: query,
			},
		}

		// Тест
		err := cache.SaveProfessionsList(ctx, professions)
		require.NoError(t, err)

		// Проверяем все поля
		result, err := cache.GetProfessionsList(ctx)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, id, result[0].ID)
		require.Equal(t, name, result[0].Name)
		require.Equal(t, query, result[0].VacancyQuery)
	})
}
