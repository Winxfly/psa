//go:build integration

// Интеграционные тесты для skill репозитория (skill_formal и skill_extracted).
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

const migrationsPathSkill = "migrations"

func mustParsePortForSkill(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForSkill(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForSkill(t, port),
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

func setupTestDBSkill(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathSkill)
	require.NoError(t, err)

	return createStorageForSkill(t, pg.DSN, pg.Host, pg.Port)
}

func cleanSkillTables(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	// Очищаем в порядке, обратном порядку создания FK
	_, err := storage.Pool.Exec(ctx, `
		TRUNCATE skill_formal, skill_extracted, stat, scraping, profession RESTART IDENTITY CASCADE
	`)
	require.NoError(t, err)
}

func createProfession(ctx context.Context, t *testing.T, storage *postgresql.Storage, name, vacancyQuery string, isActive bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO profession (id, name, vacancy_query, is_active)
		VALUES ($1, $2, $3, $4)
	`, id, name, vacancyQuery, isActive)
	require.NoError(t, err)

	return id
}

func createScrapingSessionSkill(ctx context.Context, t *testing.T, storage *postgresql.Storage, scrapedAt time.Time) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO scraping (id, scraped_at)
		VALUES ($1, $2)
	`, id, scrapedAt)
	require.NoError(t, err)

	return id
}

func TestSkillRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBSkill(t)

	t.Run("SaveFormalSkills_Success", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		// Подготовка данных
		professionID := createProfession(ctx, t, storage, "Go Developer", "go developer", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills := map[string]int{
			"Go":         10,
			"PostgreSQL": 8,
			"Docker":     5,
		}

		// Тест
		err := storage.SaveFormalSkills(ctx, sessionID, professionID, skills)

		// Assert
		require.NoError(t, err)

		// Проверяем что навыки сохранились
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID)
		require.NoError(t, err)
		require.Len(t, result, 3)

		// Проверяем данные (ORDER BY count DESC)
		require.Equal(t, "Go", result[0].Skill)
		require.Equal(t, int32(10), result[0].Count)
		require.Equal(t, "PostgreSQL", result[1].Skill)
		require.Equal(t, int32(8), result[1].Count)
		require.Equal(t, "Docker", result[2].Skill)
		require.Equal(t, int32(5), result[2].Count)
	})

	t.Run("SaveFormalSkills_EmptyMap", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Go Developer #2", "go developer 2", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills := map[string]int{}

		// Тест
		err := storage.SaveFormalSkills(ctx, sessionID, professionID, skills)

		// Assert
		require.NoError(t, err)

		// Дополнительно проверяем что в БД действительно ничего не добавилось
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("SaveExtractedSkills_Success", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		// Подготовка данных
		professionID := createProfession(ctx, t, storage, "Python Developer", "python developer", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills := map[string]int{
			"Python":     15,
			"Django":     12,
			"REST API":   9,
			"PostgreSQL": 7,
		}

		// Тест
		err := storage.SaveExtractedSkills(ctx, sessionID, professionID, skills)

		// Assert
		require.NoError(t, err)

		// Проверяем что навыки сохранились
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID)
		require.NoError(t, err)
		require.Len(t, result, 4)

		// Проверяем данные (ORDER BY count DESC)
		require.Equal(t, "Python", result[0].Skill)
		require.Equal(t, int32(15), result[0].Count)
		require.Equal(t, "Django", result[1].Skill)
		require.Equal(t, int32(12), result[1].Count)
		require.Equal(t, "REST API", result[2].Skill)
		require.Equal(t, int32(9), result[2].Count)
		require.Equal(t, "PostgreSQL", result[3].Skill)
		require.Equal(t, int32(7), result[3].Count)
	})

	t.Run("SaveExtractedSkills_EmptyMap", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Python Developer #2", "python developer 2", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills := map[string]int{}

		// Тест
		err := storage.SaveExtractedSkills(ctx, sessionID, professionID, skills)

		// Assert
		require.NoError(t, err)

		// Дополнительно проверяем что в БД действительно ничего не добавилось
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetFormalSkillsByProfessionAndDate_Success", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Java Developer", "java developer", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills := map[string]int{
			"Java":      20,
			"Spring":    15,
			"Hibernate": 10,
		}

		err := storage.SaveFormalSkills(ctx, sessionID, professionID, skills)
		require.NoError(t, err)

		// Тест
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID)

		// Assert
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, "Java", result[0].Skill)
		require.Equal(t, int32(20), result[0].Count)
		require.Equal(t, "Spring", result[1].Skill)
		require.Equal(t, int32(15), result[1].Count)
		require.Equal(t, "Hibernate", result[2].Skill)
		require.Equal(t, int32(10), result[2].Count)
	})

	t.Run("GetFormalSkillsByProfessionAndDate_Empty", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Java Developer #2", "java developer 2", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		// Тест (навыки не добавлены)
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetFormalSkillsByProfessionAndDate_WrongProfession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Java Developer #3", "java developer 3", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())
		wrongProfessionID := createProfession(ctx, t, storage, "Java Developer #4", "java developer 4", true)

		skills := map[string]int{
			"Java": 20,
		}

		err := storage.SaveFormalSkills(ctx, sessionID, professionID, skills)
		require.NoError(t, err)

		// Тест (запрашиваем для другой профессии)
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, wrongProfessionID, sessionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetFormalSkillsByProfessionAndDate_WrongSession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Java Developer #5", "java developer 5", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())
		wrongSessionID := createScrapingSessionSkill(ctx, t, storage, time.Now().Add(-1*time.Hour))

		skills := map[string]int{
			"Java": 20,
		}

		err := storage.SaveFormalSkills(ctx, sessionID, professionID, skills)
		require.NoError(t, err)

		// Тест (запрашиваем для другой сессии)
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, wrongSessionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetExtractedSkillsByProfessionAndDate_Success", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Frontend Developer", "frontend developer", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills := map[string]int{
			"React":      25,
			"TypeScript": 20,
			"CSS":        15,
			"HTML":       15,
		}

		err := storage.SaveExtractedSkills(ctx, sessionID, professionID, skills)
		require.NoError(t, err)

		// Тест
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID)

		// Assert
		require.NoError(t, err)
		require.Len(t, result, 4)
		require.Equal(t, "React", result[0].Skill)
		require.Equal(t, int32(25), result[0].Count)
		require.Equal(t, "TypeScript", result[1].Skill)
		require.Equal(t, int32(20), result[1].Count)
		// CSS и HTML имеют одинаковый count, порядок может быть любым
		require.Contains(t, []string{"CSS", "HTML"}, result[2].Skill)
		require.Equal(t, int32(15), result[2].Count)
		require.Contains(t, []string{"CSS", "HTML"}, result[3].Skill)
		require.Equal(t, int32(15), result[3].Count)
	})

	t.Run("GetExtractedSkillsByProfessionAndDate_Empty", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Frontend Developer #2", "frontend developer 2", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		// Тест (навыки не добавлены)
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetExtractedSkillsByProfessionAndDate_WrongProfession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Frontend Developer #3", "frontend developer 3", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())
		wrongProfessionID := createProfession(ctx, t, storage, "Frontend Developer #4", "frontend developer 4", true)

		skills := map[string]int{
			"React": 25,
		}

		err := storage.SaveExtractedSkills(ctx, sessionID, professionID, skills)
		require.NoError(t, err)

		// Тест (запрашиваем для другой профессии)
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, wrongProfessionID, sessionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("GetExtractedSkillsByProfessionAndDate_WrongSession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Frontend Developer #5", "frontend developer 5", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())
		wrongSessionID := createScrapingSessionSkill(ctx, t, storage, time.Now().Add(-1*time.Hour))

		skills := map[string]int{
			"React": 25,
		}

		err := storage.SaveExtractedSkills(ctx, sessionID, professionID, skills)
		require.NoError(t, err)

		// Тест (запрашиваем для другой сессии)
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, wrongSessionID)

		// Assert
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("SaveAndGetFormalSkills_MultipleSessions", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "DevOps Engineer", "devops engineer", true)
		sessionID1 := createScrapingSessionSkill(ctx, t, storage, time.Now().Add(-24*time.Hour))
		sessionID2 := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills1 := map[string]int{
			"Kubernetes": 10,
			"Docker":     8,
		}

		skills2 := map[string]int{
			"Kubernetes": 15,
			"Terraform":  12,
		}

		err := storage.SaveFormalSkills(ctx, sessionID1, professionID, skills1)
		require.NoError(t, err)

		err = storage.SaveFormalSkills(ctx, sessionID2, professionID, skills2)
		require.NoError(t, err)

		// Тест - получаем навыки для первой сессии
		result1, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID1)
		require.NoError(t, err)
		require.Len(t, result1, 2)
		require.Equal(t, "Kubernetes", result1[0].Skill)
		require.Equal(t, int32(10), result1[0].Count)
		require.Equal(t, "Docker", result1[1].Skill)
		require.Equal(t, int32(8), result1[1].Count)

		// Тест - получаем навыки для второй сессии
		result2, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID2)
		require.NoError(t, err)
		require.Len(t, result2, 2)
		require.Equal(t, "Kubernetes", result2[0].Skill)
		require.Equal(t, int32(15), result2[0].Count)
		require.Equal(t, "Terraform", result2[1].Skill)
		require.Equal(t, int32(12), result2[1].Count)
	})

	t.Run("SaveAndGetExtractedSkills_MultipleSessions", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Data Scientist", "data scientist", true)
		sessionID1 := createScrapingSessionSkill(ctx, t, storage, time.Now().Add(-24*time.Hour))
		sessionID2 := createScrapingSessionSkill(ctx, t, storage, time.Now())

		skills1 := map[string]int{
			"Python": 20,
			"ML":     15,
			"Pandas": 10,
		}

		skills2 := map[string]int{
			"Python":        25,
			"Deep Learning": 18,
			"PyTorch":       14,
		}

		err := storage.SaveExtractedSkills(ctx, sessionID1, professionID, skills1)
		require.NoError(t, err)

		err = storage.SaveExtractedSkills(ctx, sessionID2, professionID, skills2)
		require.NoError(t, err)

		// Тест - получаем навыки для первой сессии
		result1, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID1)
		require.NoError(t, err)
		require.Len(t, result1, 3)
		require.Equal(t, "Python", result1[0].Skill)
		require.Equal(t, int32(20), result1[0].Count)
		require.Equal(t, "ML", result1[1].Skill)
		require.Equal(t, int32(15), result1[1].Count)
		require.Equal(t, "Pandas", result1[2].Skill)
		require.Equal(t, int32(10), result1[2].Count)

		// Тест - получаем навыки для второй сессии
		result2, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID2)
		require.NoError(t, err)
		require.Len(t, result2, 3)
		require.Equal(t, "Python", result2[0].Skill)
		require.Equal(t, int32(25), result2[0].Count)
		require.Equal(t, "Deep Learning", result2[1].Skill)
		require.Equal(t, int32(18), result2[1].Count)
		require.Equal(t, "PyTorch", result2[2].Skill)
		require.Equal(t, int32(14), result2[2].Count)
	})

	t.Run("SaveFormalSkills_OverwriteSameSkill", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Backend Developer", "backend developer", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		// Сначала сохраняем навыки
		skills1 := map[string]int{
			"Go": 10,
		}

		err := storage.SaveFormalSkills(ctx, sessionID, professionID, skills1)
		require.NoError(t, err)

		// Затем обновляем count для того же навыка
		skills2 := map[string]int{
			"Go": 20,
		}

		err = storage.SaveFormalSkills(ctx, sessionID, professionID, skills2)
		require.NoError(t, err)

		// Тест
		result, err := storage.GetFormalSkillsByProfessionAndDate(ctx, professionID, sessionID)

		// Assert - проверяем что навык обновился (или добавился дубликат, в зависимости от реализации)
		require.NoError(t, err)
		// В текущей реализации INSERT без ON CONFLICT, поэтому будет 2 записи
		require.Len(t, result, 2)
	})

	t.Run("SaveExtractedSkills_OverwriteSameSkill", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Backend Developer #2", "backend developer 2", true)
		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())

		// Сначала сохраняем навыки
		skills1 := map[string]int{
			"Python": 15,
		}

		err := storage.SaveExtractedSkills(ctx, sessionID, professionID, skills1)
		require.NoError(t, err)

		// Затем обновляем count для того же навыка
		skills2 := map[string]int{
			"Python": 25,
		}

		err = storage.SaveExtractedSkills(ctx, sessionID, professionID, skills2)
		require.NoError(t, err)

		// Тест
		result, err := storage.GetExtractedSkillsByProfessionAndDate(ctx, professionID, sessionID)

		// Assert - проверяем что навык обновился (или добавился дубликат, в зависимости от реализации)
		require.NoError(t, err)
		// В текущей реализации INSERT без ON CONFLICT, поэтому будет 2 записи
		require.Len(t, result, 2)
	})

	t.Run("SaveFormalSkills_InvalidProfession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())
		fakeProfessionID := uuid.New()
		skills := map[string]int{"Go": 10}

		// Тест - нарушение FK (профессия не существует)
		err := storage.SaveFormalSkills(ctx, sessionID, fakeProfessionID, skills)

		// Assert - ожидаем ошибку из-за FK
		require.Error(t, err)
	})

	t.Run("SaveExtractedSkills_InvalidProfession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		sessionID := createScrapingSessionSkill(ctx, t, storage, time.Now())
		fakeProfessionID := uuid.New()
		skills := map[string]int{"Python": 15}

		// Тест - нарушение FK (профессия не существует)
		err := storage.SaveExtractedSkills(ctx, sessionID, fakeProfessionID, skills)

		// Assert - ожидаем ошибку из-за FK
		require.Error(t, err)
	})

	t.Run("SaveFormalSkills_InvalidSession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Backend Developer #3", "backend developer 3", true)
		fakeSessionID := uuid.New()
		skills := map[string]int{"Go": 10}

		// Тест - нарушение FK (сессия не существует)
		err := storage.SaveFormalSkills(ctx, fakeSessionID, professionID, skills)

		// Assert - ожидаем ошибку из-за FK
		require.Error(t, err)
	})

	t.Run("SaveExtractedSkills_InvalidSession", func(t *testing.T) {
		cleanSkillTables(ctx, t, storage)

		professionID := createProfession(ctx, t, storage, "Backend Developer #4", "backend developer 4", true)
		fakeSessionID := uuid.New()
		skills := map[string]int{"Python": 15}

		// Тест - нарушение FK (сессия не существует)
		err := storage.SaveExtractedSkills(ctx, fakeSessionID, professionID, skills)

		// Assert - ожидаем ошибку из-за FK
		require.Error(t, err)
	})
}
