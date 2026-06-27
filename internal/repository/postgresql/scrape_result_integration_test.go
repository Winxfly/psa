//go:build integration

// Интеграционные тесты для атомарного сохранения scraping result.
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

const migrationsPathScrapeResult = "migrations"

func mustParsePortForScrapeResult(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForScrapeResult(t *testing.T, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForScrapeResult(t, port),
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

func setupTestDBScrapeResult(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathScrapeResult)
	require.NoError(t, err)

	return createStorageForScrapeResult(t, pg.Host, pg.Port)
}

func cleanScrapeResultTables(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()

	_, err := storage.Pool.Exec(ctx, `
		TRUNCATE skill_corpus, skill_formal, skill_extracted, stat, stat_daily, scraping, profession
		RESTART IDENTITY CASCADE
	`)
	require.NoError(t, err)
}

func createProfessionForScrapeResult(ctx context.Context, t *testing.T, storage *postgresql.Storage,
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

func createScrapingSessionForScrapeResult(ctx context.Context, t *testing.T, storage *postgresql.Storage,
	scrapedAt time.Time) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO scraping (id, scraped_at)
		VALUES ($1, $2)
	`, id, scrapedAt)
	require.NoError(t, err)

	return id
}

func insertSkillCorpusForScrapeResult(ctx context.Context, t *testing.T, storage *postgresql.Storage,
	professionID uuid.UUID, skill string, count int) {
	t.Helper()

	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO skill_corpus (profession_id, skill, mention_count)
		VALUES ($1, $2, $3)
	`, professionID, skill, count)
	require.NoError(t, err)
}

func countRowsForScrapeResult(ctx context.Context, t *testing.T, storage *postgresql.Storage, query string, args ...any) int {
	t.Helper()

	var count int
	err := storage.Pool.QueryRow(ctx, query, args...).Scan(&count)
	require.NoError(t, err)

	return count
}

func getStatDailyScrapedAtForScrapeResult(ctx context.Context, t *testing.T,
	storage *postgresql.Storage, professionID uuid.UUID) time.Time {
	t.Helper()

	var scrapedAt time.Time
	err := storage.Pool.QueryRow(ctx, `
		SELECT scraped_at
		FROM stat_daily
		WHERE profession_id = $1
		ORDER BY scraped_at DESC
		LIMIT 1
	`, professionID).Scan(&scrapedAt)
	require.NoError(t, err)

	return scrapedAt
}

func TestScrapeResultRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBScrapeResult(t)

	t.Run("SaveArchiveProfessionResult_Success", func(t *testing.T) {
		cleanScrapeResultTables(ctx, t, storage)

		professionID := createProfessionForScrapeResult(ctx, t, storage,
			"Go Developer SaveArchiveProfessionResult_Success", "go developer", true)
		sessionID := createScrapingSessionForScrapeResult(ctx, t, storage, time.Now())
		scrapedAt := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)

		result := domain.ProfessionScrapeResult{
			ProfessionID: professionID,
			VacancyCount: 150,
			FormalSkills: map[string]int{
				"Go":         10,
				"PostgreSQL": 7,
			},
			ExtractedSkills: map[string]int{
				"Docker": 5,
			},
		}

		err := storage.SaveArchiveProfessionResult(ctx, sessionID, scrapedAt, result)
		require.NoError(t, err)

		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM stat WHERE profession_id = $1 AND scraped_at_id = $2`, professionID, sessionID))
		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM stat_daily WHERE profession_id = $1`, professionID))
		require.Equal(t, 2, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_formal WHERE profession_id = $1 AND scraped_at_id = $2`, professionID, sessionID))
		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_extracted WHERE profession_id = $1 AND scraped_at_id = $2`, professionID, sessionID))
		require.WithinDuration(t, scrapedAt, getStatDailyScrapedAtForScrapeResult(ctx, t, storage, professionID),
			time.Microsecond)

		corpus, err := storage.GetActiveSkillCorpus(ctx)
		require.NoError(t, err)
		require.Len(t, corpus[professionID], 2)
		require.Equal(t, 10, corpus[professionID]["Go"])
		require.Equal(t, 7, corpus[professionID]["PostgreSQL"])
	})

	t.Run("SaveArchiveProfessionResult_ReplacesSkillCorpus", func(t *testing.T) {
		cleanScrapeResultTables(ctx, t, storage)

		professionID := createProfessionForScrapeResult(ctx, t, storage,
			"Go Developer SaveArchiveProfessionResult_ReplacesSkillCorpus", "go developer", true)
		sessionID := createScrapingSessionForScrapeResult(ctx, t, storage, time.Now())
		scrapedAt := time.Date(2026, 6, 24, 11, 0, 0, 0, time.UTC)
		insertSkillCorpusForScrapeResult(ctx, t, storage, professionID, "Old Skill", 99)

		result := domain.ProfessionScrapeResult{
			ProfessionID: professionID,
			VacancyCount: 120,
			FormalSkills: map[string]int{
				"Go": 10,
			},
			ExtractedSkills: map[string]int{
				"Docker": 5,
			},
		}

		err := storage.SaveArchiveProfessionResult(ctx, sessionID, scrapedAt, result)
		require.NoError(t, err)

		corpus, err := storage.GetActiveSkillCorpus(ctx)
		require.NoError(t, err)
		require.Len(t, corpus[professionID], 1)
		require.NotContains(t, corpus[professionID], "Old Skill")
		require.Equal(t, 10, corpus[professionID]["Go"])
	})

	t.Run("SaveArchiveProfessionResult_RollsBackOnDuplicateResult", func(t *testing.T) {
		cleanScrapeResultTables(ctx, t, storage)

		professionID := createProfessionForScrapeResult(ctx, t, storage,
			"Go Developer SaveArchiveProfessionResult_RollsBackOnDuplicateResult", "go developer", true)
		sessionID := createScrapingSessionForScrapeResult(ctx, t, storage, time.Now())
		firstScrapedAt := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
		secondScrapedAt := time.Date(2026, 6, 24, 13, 0, 0, 0, time.UTC)

		firstResult := domain.ProfessionScrapeResult{
			ProfessionID: professionID,
			VacancyCount: 150,
			FormalSkills: map[string]int{
				"Go": 10,
			},
			ExtractedSkills: map[string]int{
				"REST": 5,
			},
		}
		secondResult := domain.ProfessionScrapeResult{
			ProfessionID: professionID,
			VacancyCount: 999,
			FormalSkills: map[string]int{
				"Java": 20,
			},
			ExtractedSkills: map[string]int{
				"GraphQL": 30,
			},
		}

		err := storage.SaveArchiveProfessionResult(ctx, sessionID, firstScrapedAt, firstResult)
		require.NoError(t, err)

		err = storage.SaveArchiveProfessionResult(ctx, sessionID, secondScrapedAt, secondResult)
		require.Error(t, err)

		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM stat WHERE profession_id = $1 AND scraped_at_id = $2`, professionID, sessionID))
		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM stat_daily WHERE profession_id = $1`, professionID))
		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_formal WHERE profession_id = $1 AND scraped_at_id = $2`, professionID, sessionID))
		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_extracted WHERE profession_id = $1 AND scraped_at_id = $2`, professionID, sessionID))
		require.Equal(t, 0, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_formal WHERE profession_id = $1 AND scraped_at_id = $2 AND skill = 'Java'`,
			professionID, sessionID))
		require.Equal(t, 0, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_extracted WHERE profession_id = $1 AND scraped_at_id = $2 AND skill = 'GraphQL'`,
			professionID, sessionID))

		corpus, err := storage.GetActiveSkillCorpus(ctx)
		require.NoError(t, err)
		require.Len(t, corpus[professionID], 1)
		require.Equal(t, 10, corpus[professionID]["Go"])
		require.NotContains(t, corpus[professionID], "Java")
		require.WithinDuration(t, firstScrapedAt, getStatDailyScrapedAtForScrapeResult(ctx, t, storage, professionID),
			time.Microsecond)
	})

	t.Run("SaveDailyProfessionResult_Success", func(t *testing.T) {
		cleanScrapeResultTables(ctx, t, storage)

		professionID := createProfessionForScrapeResult(ctx, t, storage,
			"Go Developer SaveDailyProfessionResult_Success", "go developer", true)
		scrapedAt := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)

		result := domain.ProfessionScrapeResult{
			ProfessionID: professionID,
			VacancyCount: 80,
			FormalSkills: map[string]int{
				"Go":         10,
				"PostgreSQL": 7,
			},
			ExtractedSkills: map[string]int{
				"Docker": 5,
			},
		}

		err := storage.SaveDailyProfessionResult(ctx, scrapedAt, result)
		require.NoError(t, err)

		require.Equal(t, 1, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM stat_daily WHERE profession_id = $1`, professionID))
		require.WithinDuration(t, scrapedAt, getStatDailyScrapedAtForScrapeResult(ctx, t, storage, professionID),
			time.Microsecond)
		require.Equal(t, 0, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM stat WHERE profession_id = $1`, professionID))
		require.Equal(t, 0, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_formal WHERE profession_id = $1`, professionID))
		require.Equal(t, 0, countRowsForScrapeResult(ctx, t, storage,
			`SELECT count(*) FROM skill_extracted WHERE profession_id = $1`, professionID))

		corpus, err := storage.GetActiveSkillCorpus(ctx)
		require.NoError(t, err)
		require.Len(t, corpus[professionID], 2)
		require.Equal(t, 10, corpus[professionID]["Go"])
		require.Equal(t, 7, corpus[professionID]["PostgreSQL"])
	})

	t.Run("SaveDailyProfessionResult_ReplacesSkillCorpus", func(t *testing.T) {
		cleanScrapeResultTables(ctx, t, storage)

		professionID := createProfessionForScrapeResult(ctx, t, storage,
			"Go Developer SaveDailyProfessionResult_ReplacesSkillCorpus", "go developer", true)
		insertSkillCorpusForScrapeResult(ctx, t, storage, professionID, "Old Skill", 99)

		result := domain.ProfessionScrapeResult{
			ProfessionID: professionID,
			VacancyCount: 80,
			FormalSkills: map[string]int{
				"Go": 10,
			},
			ExtractedSkills: map[string]int{
				"Docker": 5,
			},
		}

		err := storage.SaveDailyProfessionResult(ctx, time.Now(), result)
		require.NoError(t, err)

		corpus, err := storage.GetActiveSkillCorpus(ctx)
		require.NoError(t, err)
		require.Len(t, corpus[professionID], 1)
		require.NotContains(t, corpus[professionID], "Old Skill")
		require.Equal(t, 10, corpus[professionID]["Go"])
	})
}
