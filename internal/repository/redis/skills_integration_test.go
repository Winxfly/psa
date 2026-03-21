//go:build integration

// Интеграционные тесты для redis skills репозитория.
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

// clientTestSkills returns the underlying redis client for testing purposes.
// This method is only available in integration tests.
func (c *Cache) clientTestSkills() *redis.Client {
	return c.client
}

func createCacheForSkills(t *testing.T, addr string) *Cache {
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

func setupTestRedisSkills(t *testing.T) *Cache {
	t.Helper()

	ctx := context.Background()
	redisContainer, err := containers.StartRedis(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = redisContainer.Container.Terminate(ctx)
	})

	return createCacheForSkills(t, redisContainer.Addr)
}

func cleanSkillsCache(ctx context.Context, t *testing.T, cache *Cache, professionID uuid.UUID) {
	t.Helper()
	key := fmt.Sprintf(ProfessionSkillsKeyPrefix, professionID.String())
	err := cache.clientTestSkills().Del(ctx, key).Err()
	require.NoError(t, err)
}

func createProfessionDetail(professionID uuid.UUID, name string) *domain.ProfessionDetail {
	return &domain.ProfessionDetail{
		ProfessionID:   professionID,
		ProfessionName: name,
		ScrapedAt:      "2023-01-01T10:00:00Z",
		VacancyCount:   150,
		FormalSkills: []domain.SkillResponse{
			{Skill: "Go", Count: 100},
			{Skill: "PostgreSQL", Count: 80},
		},
		ExtractedSkills: []domain.SkillResponse{
			{Skill: "REST API", Count: 50},
			{Skill: "Docker", Count: 40},
		},
	}
}

func TestSkillsCache(t *testing.T) {
	ctx := context.Background()
	cache := setupTestRedisSkills(t)

	t.Run("SaveProfessionData_Success", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		data := createProfessionDetail(professionID, "Go Developer")

		// Тест
		err := cache.SaveProfessionData(ctx, data)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Проверяем каждое поле детально
		require.Equal(t, professionID, result.ProfessionID)
		require.Equal(t, "Go Developer", result.ProfessionName)
		require.Equal(t, "2023-01-01T10:00:00Z", result.ScrapedAt)
		require.Equal(t, int32(150), result.VacancyCount)
		require.Len(t, result.FormalSkills, 2)
		require.Len(t, result.ExtractedSkills, 2)

		// Проверяем навыки
		require.Equal(t, "Go", result.FormalSkills[0].Skill)
		require.Equal(t, int32(100), result.FormalSkills[0].Count)
		require.Equal(t, "PostgreSQL", result.FormalSkills[1].Skill)
		require.Equal(t, int32(80), result.FormalSkills[1].Count)

		require.Equal(t, "REST API", result.ExtractedSkills[0].Skill)
		require.Equal(t, int32(50), result.ExtractedSkills[0].Count)
		require.Equal(t, "Docker", result.ExtractedSkills[1].Skill)
		require.Equal(t, int32(40), result.ExtractedSkills[1].Count)
	})

	t.Run("SaveProfessionData_EmptySkills", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		data := &domain.ProfessionDetail{
			ProfessionID:    professionID,
			ProfessionName:  "Empty Skills Developer",
			ScrapedAt:       "2023-01-01T10:00:00Z",
			VacancyCount:    50,
			FormalSkills:    []domain.SkillResponse{},
			ExtractedSkills: []domain.SkillResponse{},
		}

		// Тест
		err := cache.SaveProfessionData(ctx, data)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные сохранились с пустыми списками
		result, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, result.FormalSkills)
		require.Empty(t, result.ExtractedSkills)
	})

	t.Run("SaveProfessionData_Overwrite", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		// Сохраняем первые данные
		data1 := createProfessionDetail(professionID, "Go Developer v1")
		err := cache.SaveProfessionData(ctx, data1)
		require.NoError(t, err)

		// Проверяем
		result1, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.Equal(t, "Go Developer v1", result1.ProfessionName)

		// Перезаписываем новыми данными
		data2 := createProfessionDetail(professionID, "Go Developer v2")
		data2.VacancyCount = 300
		data2.FormalSkills = append(data2.FormalSkills, domain.SkillResponse{Skill: "Kubernetes", Count: 60})

		err = cache.SaveProfessionData(ctx, data2)
		require.NoError(t, err)

		// Проверяем что данные перезаписались
		result2, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.Equal(t, "Go Developer v2", result2.ProfessionName)
		require.Equal(t, int32(300), result2.VacancyCount)
		require.Len(t, result2.FormalSkills, 3)
	})

	t.Run("GetProfessionData_NotFound", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		// Тест (ключ не существует)
		result, err := cache.GetProfessionData(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("SaveProfessionData_TTL", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		data := createProfessionDetail(professionID, "TTL Test Developer")

		// Тест
		err := cache.SaveProfessionData(ctx, data)
		require.NoError(t, err)

		// Проверяем TTL ключа
		key := fmt.Sprintf(ProfessionSkillsKeyPrefix, professionID.String())
		ttl, err := cache.clientTestSkills().TTL(ctx, key).Result()
		require.NoError(t, err)

		// TTL должен быть установлен (~24h)
		// Используем диапазон с допуском 1 час для стабильности
		require.GreaterOrEqual(t, ttl, 23*time.Hour)
		require.LessOrEqual(t, ttl, 24*time.Hour)
	})

	t.Run("SaveProfessionData_Expiration", func(t *testing.T) {
		// Тест на реальное истечение TTL
		// Создаем кэш с очень маленьким TTL для быстрого теста

		cfg := config.Redis{
			Addr:       cache.clientTestSkills().Options().Addr,
			Password:   "",
			DB:         0,
			DefaultTTL: 2 * time.Second, // TTL = 2 секунды
		}

		shortTTLCache, err := New(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			shortTTLCache.Close()
		})

		professionID := uuid.New()
		data := createProfessionDetail(professionID, "Expiring Developer")

		// Сохраняем
		err = shortTTLCache.SaveProfessionData(ctx, data)
		require.NoError(t, err)

		// Проверяем что данные есть
		result1, err := shortTTLCache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result1)

		// Ждем истечения TTL
		time.Sleep(3 * time.Second)

		// Проверяем что данные исчезли
		result2, err := shortTTLCache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.Nil(t, result2)
	})

	t.Run("GetProfessionData_InvalidJSON", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		// Записываем невалидный JSON напрямую в Redis
		key := fmt.Sprintf(ProfessionSkillsKeyPrefix, professionID.String())
		err := cache.clientTestSkills().Set(ctx, key, "invalid-json", time.Hour).Err()
		require.NoError(t, err)

		// Тест
		result, err := cache.GetProfessionData(ctx, professionID)

		// Assert - должна быть ошибка парсинга
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("SaveProfessionData_SingleSkill", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		data := &domain.ProfessionDetail{
			ProfessionID:    professionID,
			ProfessionName:  "Single Skill Developer",
			ScrapedAt:       "2023-01-01T10:00:00Z",
			VacancyCount:    25,
			FormalSkills:    []domain.SkillResponse{{Skill: "Go", Count: 1}},
			ExtractedSkills: []domain.SkillResponse{},
		}

		// Тест
		err := cache.SaveProfessionData(ctx, data)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.FormalSkills, 1)
		require.Equal(t, "Go", result.FormalSkills[0].Skill)
		require.Equal(t, int32(1), result.FormalSkills[0].Count)
	})

	t.Run("SaveProfessionData_ManySkills", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		// Создаем много навыков
		formalSkills := make([]domain.SkillResponse, 50)
		extractedSkills := make([]domain.SkillResponse, 50)
		for i := 0; i < 50; i++ {
			formalSkills[i] = domain.SkillResponse{
				Skill: fmt.Sprintf("FormalSkill_%d", i),
				Count: int32(i * 10),
			}
			extractedSkills[i] = domain.SkillResponse{
				Skill: fmt.Sprintf("ExtractedSkill_%d", i),
				Count: int32(i * 5),
			}
		}

		data := &domain.ProfessionDetail{
			ProfessionID:    professionID,
			ProfessionName:  "Many Skills Developer",
			ScrapedAt:       "2023-01-01T10:00:00Z",
			VacancyCount:    500,
			FormalSkills:    formalSkills,
			ExtractedSkills: extractedSkills,
		}

		// Тест
		err := cache.SaveProfessionData(ctx, data)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные действительно сохранились
		result, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.FormalSkills, 50)
		require.Len(t, result.ExtractedSkills, 50)
	})

	t.Run("SaveProfessionData_SpecialCharacters", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		// Данные со специальными символами, UTF-8, эмодзи
		data := &domain.ProfessionDetail{
			ProfessionID:   professionID,
			ProfessionName: "Разработчик 🐍 (Python) <Senior>",
			ScrapedAt:      "2023-01-01T10:00:00Z",
			VacancyCount:   100,
			FormalSkills: []domain.SkillResponse{
				{Skill: "Python & Django", Count: 80},
				{Skill: "SQL \"Advanced\"", Count: 60},
				{Skill: "Linux\nBash\tScript", Count: 40},
			},
			ExtractedSkills: []domain.SkillResponse{
				{Skill: "REST API's", Count: 50},
				{Skill: "C++/CLI", Count: 30},
			},
		}

		// Тест
		err := cache.SaveProfessionData(ctx, data)

		// Assert
		require.NoError(t, err)

		// Проверяем что данные сохранились корректно
		result, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "Разработчик 🐍 (Python) <Senior>", result.ProfessionName)
		require.Len(t, result.FormalSkills, 3)
		require.Len(t, result.ExtractedSkills, 2)
	})

	t.Run("SaveAndGetProfessionData_VerifyFields", func(t *testing.T) {
		professionID := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID)
		})

		name := "Test Profession Name"
		scrapedAt := "2023-06-15T14:30:00Z"
		vacancyCount := int32(999)

		data := &domain.ProfessionDetail{
			ProfessionID:   professionID,
			ProfessionName: name,
			ScrapedAt:      scrapedAt,
			VacancyCount:   vacancyCount,
			FormalSkills: []domain.SkillResponse{
				{Skill: "Skill A", Count: 10},
				{Skill: "Skill B", Count: 20},
			},
			ExtractedSkills: []domain.SkillResponse{
				{Skill: "Skill C", Count: 30},
			},
		}

		// Тест
		err := cache.SaveProfessionData(ctx, data)
		require.NoError(t, err)

		// Проверяем все поля
		result, err := cache.GetProfessionData(ctx, professionID)
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Equal(t, professionID, result.ProfessionID)
		require.Equal(t, name, result.ProfessionName)
		require.Equal(t, scrapedAt, result.ScrapedAt)
		require.Equal(t, vacancyCount, result.VacancyCount)
		require.Len(t, result.FormalSkills, 2)
		require.Len(t, result.ExtractedSkills, 1)
	})

	t.Run("SaveProfessionData_MultipleProfessions", func(t *testing.T) {
		professionID1 := uuid.New()
		professionID2 := uuid.New()
		t.Cleanup(func() {
			cleanSkillsCache(ctx, t, cache, professionID1)
			cleanSkillsCache(ctx, t, cache, professionID2)
		})

		// Сохраняем данные для двух профессий
		data1 := createProfessionDetail(professionID1, "Go Developer")
		data2 := createProfessionDetail(professionID2, "Python Developer")
		data2.VacancyCount = 200

		err := cache.SaveProfessionData(ctx, data1)
		require.NoError(t, err)

		err = cache.SaveProfessionData(ctx, data2)
		require.NoError(t, err)

		// Проверяем что данные не пересеклись
		result1, err := cache.GetProfessionData(ctx, professionID1)
		require.NoError(t, err)
		require.Equal(t, "Go Developer", result1.ProfessionName)
		require.Equal(t, int32(150), result1.VacancyCount)

		result2, err := cache.GetProfessionData(ctx, professionID2)
		require.NoError(t, err)
		require.Equal(t, "Python Developer", result2.ProfessionName)
		require.Equal(t, int32(200), result2.VacancyCount)
	})
}
