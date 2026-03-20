//go:build integration

// Интеграционные тесты для profession репозитория.
// Каждый тест поднимает свой контейнер для полной изоляции.
package postgresql_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"psa/internal/config"
	"psa/internal/domain"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPathProfession = "migrations"

func mustParsePortForProfession(t *testing.T, portStr string) int {
	t.Helper()

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

func createStorageForProfession(t *testing.T, dsn string, host string, port string) *postgresql.Storage {
	t.Helper()

	cfg := config.StoragePath{
		Username: "test",
		Password: "test",
		Host:     host,
		Port:     mustParsePortForProfession(t, port),
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

func setupTestDBProfession(t *testing.T) *postgresql.Storage {
	t.Helper()

	ctx := context.Background()
	pg, err := containers.StartPostgres(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(ctx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPathProfession)
	require.NoError(t, err)

	return createStorageForProfession(t, pg.DSN, pg.Host, pg.Port)
}

func cleanProfessionTable(ctx context.Context, t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(ctx, `TRUNCATE profession RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func insertProfession(ctx context.Context, t *testing.T, storage *postgresql.Storage, p domain.Profession) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(ctx, `
		INSERT INTO profession (id, name, vacancy_query, is_active)
		VALUES ($1, $2, $3, $4)
	`, id, p.Name, p.VacancyQuery, p.IsActive)
	require.NoError(t, err)

	return id
}

func TestProfessionRepository(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDBProfession(t)

	t.Run("GetActiveProfessions_Success", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создание тестовых профессий с уникальными именами
		professionID1 := uuid.New()
		professionID2 := uuid.New()
		_, err := storage.Pool.Exec(ctx, `
			INSERT INTO profession (id, name, vacancy_query, is_active)
			VALUES
				($1, $2, $3, true),
				($4, $5, $6, true),
				($7, $8, $9, false)
		`,
			professionID1, "Test Go Developer #1", "go developer test 1",
			professionID2, "Test Python Developer #1", "python developer test 1",
			uuid.New(), "Test Java Developer #1", "java developer test 1",
		)
		require.NoError(t, err)

		// Тест
		professions, err := storage.GetActiveProfessions(ctx)

		// Assert
		require.NoError(t, err)
		require.Len(t, professions, 2)

		// Проверяем что все активные
		for _, p := range professions {
			require.True(t, p.IsActive)
		}
	})

	t.Run("GetActiveProfessions_Empty", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Тест (нет профессий)
		professions, err := storage.GetActiveProfessions(ctx)

		// Assert
		require.NoError(t, err)
		require.Empty(t, professions)
	})

	t.Run("GetAllProfessions_Success", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создание тестовых профессий с уникальными именами
		_, err := storage.Pool.Exec(ctx, `
			INSERT INTO profession (id, name, vacancy_query, is_active)
			VALUES
				($1, $2, $3, true),
				($4, $5, $6, false)
		`,
			uuid.New(), "Test All Go Developer #2", "go developer test 2",
			uuid.New(), "Test All Python Developer #2", "python developer test 2",
		)
		require.NoError(t, err)

		// Тест
		professions, err := storage.GetAllProfessions(ctx)

		// Assert
		require.NoError(t, err)
		require.Len(t, professions, 2)

		// Проверяем имена
		names := []string{professions[0].Name, professions[1].Name}
		require.Contains(t, names, "Test All Go Developer #2")
		require.Contains(t, names, "Test All Python Developer #2")
	})

	t.Run("GetProfessionByID_Success", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создание тестовой профессии с уникальным именем
		professionID := uuid.New()
		_, err := storage.Pool.Exec(ctx, `
			INSERT INTO profession (id, name, vacancy_query, is_active)
			VALUES ($1, $2, $3, true)
		`, professionID, "Test Get By ID Go Developer #3", "go developer test 3")
		require.NoError(t, err)

		// Тест
		profession, err := storage.GetProfessionByID(ctx, professionID)

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID, profession.ID)
		require.Equal(t, "Test Get By ID Go Developer #3", profession.Name)
		require.Equal(t, "go developer test 3", profession.VacancyQuery)
		require.True(t, profession.IsActive)
	})

	t.Run("GetProfessionByID_NotFound", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Тест (несуществующий ID)
		nonExistentID := uuid.New()
		profession, err := storage.GetProfessionByID(ctx, nonExistentID)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionNotFound)
		require.Empty(t, profession)
	})

	t.Run("GetProfessionByName_Success", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создание тестовой профессии с уникальным именем
		professionID := uuid.New()
		_, err := storage.Pool.Exec(ctx, `
			INSERT INTO profession (id, name, vacancy_query, is_active)
			VALUES ($1, $2, $3, true)
		`, professionID, "Test Get By Name Go Developer #4", "go developer test 4")
		require.NoError(t, err)

		// Тест
		profession, err := storage.GetProfessionByName(ctx, "Test Get By Name Go Developer #4")

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID, profession.ID)
		require.Equal(t, "Test Get By Name Go Developer #4", profession.Name)
		require.True(t, profession.IsActive)
	})

	t.Run("GetProfessionByName_NotFound", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Тест (несуществующее имя)
		profession, err := storage.GetProfessionByName(ctx, "Nonexistent Profession")

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionNotFound)
		require.Empty(t, profession)
	})

	t.Run("AddProfession_Success", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Тест - добавляем профессию с уникальным именем
		profession := domain.Profession{
			Name:         "Test Add Go Developer #5",
			VacancyQuery: "go developer test 5",
			IsActive:     true,
		}

		professionID, err := storage.AddProfession(ctx, profession)

		// Assert
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, professionID)

		// Проверяем что профессия действительно создана
		created, err := storage.GetProfessionByID(ctx, professionID)
		require.NoError(t, err)
		require.Equal(t, profession.Name, created.Name)
		require.Equal(t, profession.VacancyQuery, created.VacancyQuery)
	})

	t.Run("AddProfession_AlreadyExists", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создание первой профессии с уникальным именем
		profession := domain.Profession{
			Name:         "Test Add Duplicate Go Developer #6",
			VacancyQuery: "go developer test 6",
			IsActive:     true,
		}

		insertProfession(ctx, t, storage, profession)

		// Тест - попытка добавить такую же профессию (по имени)
		professionID, err := storage.AddProfession(ctx, profession)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionAlreadyExists)
		require.Equal(t, uuid.Nil, professionID)
	})

	t.Run("UpdateProfession_Success", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создание тестовой профессии с уникальным именем
		profession := domain.Profession{
			Name:         "Test Update Go Developer #7",
			VacancyQuery: "go developer test 7",
			IsActive:     true,
		}
		professionID := insertProfession(ctx, t, storage, profession)

		// Тест - обновление
		updatedProfession := domain.Profession{
			ID:           professionID,
			Name:         "Test Updated Senior Go Developer #7",
			VacancyQuery: "senior go developer test 7",
			IsActive:     false,
		}

		err := storage.UpdateProfession(ctx, updatedProfession)

		// Assert
		require.NoError(t, err)

		// Проверяем что профессия обновилась
		updated, err := storage.GetProfessionByID(ctx, professionID)
		require.NoError(t, err)
		require.Equal(t, "Test Updated Senior Go Developer #7", updated.Name)
		require.Equal(t, "senior go developer test 7", updated.VacancyQuery)
		require.False(t, updated.IsActive)
	})

	t.Run("UpdateProfession_NotFound", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Тест - обновление несуществующей профессии
		nonExistentID := uuid.New()
		profession := domain.Profession{
			ID:           nonExistentID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		}

		err := storage.UpdateProfession(ctx, profession)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionNotFound)
	})

	t.Run("UpdateProfession_AlreadyExists", func(t *testing.T) {
		cleanProfessionTable(ctx, t, storage)

		// Создаем две профессии
		profession1 := domain.Profession{
			Name:         "Test Update Unique #8",
			VacancyQuery: "test 8",
			IsActive:     true,
		}
		profession2 := domain.Profession{
			Name:         "Test Update Target #9",
			VacancyQuery: "test 9",
			IsActive:     true,
		}

		insertProfession(ctx, t, storage, profession1)
		id2 := insertProfession(ctx, t, storage, profession2)

		// Пытаемся обновить profession2 именем profession1 (должен быть unique violation)
		profession2.ID = id2
		profession2.Name = profession1.Name

		err := storage.UpdateProfession(ctx, profession2)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionAlreadyExists)
	})
}
