//go:build integration

// Интеграционные тесты для skill_corpus репозитория.
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
	"psa/internal/domain"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPathSkillCorpus = "migrations"

func mustParsePortForSkillCorpus(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForSkillCorpus(t *testing.T, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForSkillCorpus(t, port),
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

func setupTestDBSkillCorpus(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathSkillCorpus)
	require.NoError(t, err)

	return createStorageForSkillCorpus(t, pg.Host, pg.Port)
}

func cleanSkillCorpusTables(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()

	_, err := storage.Pool.Exec(ctx, `
		TRUNCATE skill_corpus, stat_daily, profession
		RESTART IDENTITY CASCADE
	`)
	require.NoError(t, err)
}

func createProfessionForSkillCorpus(ctx context.Context, t *testing.T, storage *postgresql.Storage,
	name string, vacancyQuery string, isActive bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO profession (id, name, vacancy_query, is_active)
		VALUES ($1, $2, $3, $4)
	`, id, name, vacancyQuery, isActive)
	require.NoError(t, err)

	return id
}

func insertSkillCorpus(ctx context.Context, t *testing.T, storage *postgresql.Storage,
	professionID uuid.UUID, skill string, count int) {
	t.Helper()

	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO skill_corpus (profession_id, skill, mention_count)
		VALUES ($1, $2, $3)
	`, professionID, skill, count)
	require.NoError(t, err)
}

func TestSkillCorpusRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBSkillCorpus(t)

	t.Run("GetActiveSkillCorpus_Empty", func(t *testing.T) {
		cleanSkillCorpusTables(ctx, t, storage)

		corpus, err := storage.GetActiveSkillCorpus(ctx)

		require.NoError(t, err)
		require.Empty(t, corpus)
	})

	t.Run("GetActiveSkillCorpus_ReturnsOnlyActiveProfessions", func(t *testing.T) {
		cleanSkillCorpusTables(ctx, t, storage)

		activeID := createProfessionForSkillCorpus(ctx, t, storage,
			"Go Developer GetActiveSkillCorpus_ReturnsOnlyActiveProfessions", "go developer", true)
		inactiveID := createProfessionForSkillCorpus(ctx, t, storage,
			"Java Developer GetActiveSkillCorpus_ReturnsOnlyActiveProfessions", "java developer", false)

		insertSkillCorpus(ctx, t, storage, activeID, "Go", 10)
		insertSkillCorpus(ctx, t, storage, activeID, "PostgreSQL", 7)
		insertSkillCorpus(ctx, t, storage, inactiveID, "Java", 20)

		corpus, err := storage.GetActiveSkillCorpus(ctx)

		require.NoError(t, err)
		require.Len(t, corpus, 1)
		require.Contains(t, corpus, activeID)
		require.NotContains(t, corpus, inactiveID)
		require.Equal(t, 10, corpus[activeID]["Go"])
		require.Equal(t, 7, corpus[activeID]["PostgreSQL"])
	})

	t.Run("SaveDailyProfessionResult_EmptyFormalSkillsClearsCorpus", func(t *testing.T) {
		cleanSkillCorpusTables(ctx, t, storage)

		professionID := createProfessionForSkillCorpus(ctx, t, storage,
			"Go Developer SaveDailyProfessionResult_EmptyFormalSkillsClearsCorpus", "go developer", true)
		insertSkillCorpus(ctx, t, storage, professionID, "Old Skill", 99)

		result := domain.ProfessionScrapeResult{
			ProfessionID:    professionID,
			VacancyCount:    10,
			FormalSkills:    map[string]int{},
			ExtractedSkills: map[string]int{"Docker": 5},
		}

		err := storage.SaveDailyProfessionResult(ctx, time.Now(), result)
		require.NoError(t, err)

		corpus, err := storage.GetActiveSkillCorpus(ctx)
		require.NoError(t, err)
		require.NotContains(t, corpus, professionID)
	})
}
