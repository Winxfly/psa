//go:build integration

package postgresql_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/repository/postgresql"
	"psa/tests/containers"
)

const migrationsPath = "migrations"

// setupTestDB поднимает БД один раз на все тесты в пакете
var (
	testStorage *postgresql.Storage
	testCtx     = context.Background()
)

func setupTestDB(t *testing.T) *postgresql.Storage {
	t.Helper()

	if testStorage != nil {
		return testStorage
	}

	pg, err := containers.StartPostgres(testCtx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pg.Container.Terminate(testCtx)
	})

	err = containers.RunMigrations(pg.DSN, migrationsPath)
	require.NoError(t, err)

	testStorage = createStorage(t, pg.DSN, pg.Host, pg.Port)
	return testStorage
}

func cleanProfessionTable(t *testing.T, storage *postgresql.Storage) {
	t.Helper()
	_, err := storage.Pool.Exec(testCtx, `TRUNCATE profession RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func insertProfession(t *testing.T, storage *postgresql.Storage, p domain.Profession) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := storage.Pool.Exec(testCtx, `
		INSERT INTO profession (id, name, vacancy_query, is_active)
		VALUES ($1, $2, $3, $4)
	`, id, p.Name, p.VacancyQuery, p.IsActive)
	require.NoError(t, err)

	return id
}

func TestProfessionRepository(t *testing.T) {
	storage := setupTestDB(t)

	t.Run("GetActiveProfessions_Success", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Создание тестовых профессий с уникальными именами
		professionID1 := uuid.New()
		professionID2 := uuid.New()
		_, err := storage.Pool.Exec(testCtx, `
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
		professions, err := storage.GetActiveProfessions(testCtx)

		// Assert
		require.NoError(t, err)
		require.Len(t, professions, 2)

		// Проверяем что все активные
		for _, p := range professions {
			require.True(t, p.IsActive)
		}
	})

	t.Run("GetActiveProfessions_Empty", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Тест (нет профессий)
		professions, err := storage.GetActiveProfessions(testCtx)

		// Assert
		require.NoError(t, err)
		require.Empty(t, professions)
	})

	t.Run("GetAllProfessions_Success", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Создание тестовых профессий с уникальными именами
		_, err := storage.Pool.Exec(testCtx, `
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
		professions, err := storage.GetAllProfessions(testCtx)

		// Assert
		require.NoError(t, err)
		require.Len(t, professions, 2)

		// Проверяем имена
		names := []string{professions[0].Name, professions[1].Name}
		require.Contains(t, names, "Test All Go Developer #2")
		require.Contains(t, names, "Test All Python Developer #2")
	})

	t.Run("GetProfessionByID_Success", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Создание тестовой профессии с уникальным именем
		professionID := uuid.New()
		_, err := storage.Pool.Exec(testCtx, `
			INSERT INTO profession (id, name, vacancy_query, is_active)
			VALUES ($1, $2, $3, true)
		`, professionID, "Test Get By ID Go Developer #3", "go developer test 3")
		require.NoError(t, err)

		// Тест
		profession, err := storage.GetProfessionByID(testCtx, professionID)

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID, profession.ID)
		require.Equal(t, "Test Get By ID Go Developer #3", profession.Name)
		require.Equal(t, "go developer test 3", profession.VacancyQuery)
		require.True(t, profession.IsActive)
	})

	t.Run("GetProfessionByID_NotFound", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Тест (несуществующий ID)
		nonExistentID := uuid.New()
		profession, err := storage.GetProfessionByID(testCtx, nonExistentID)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionNotFound)
		require.Empty(t, profession)
	})

	t.Run("GetProfessionByName_Success", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Создание тестовой профессии с уникальным именем
		professionID := uuid.New()
		_, err := storage.Pool.Exec(testCtx, `
			INSERT INTO profession (id, name, vacancy_query, is_active)
			VALUES ($1, $2, $3, true)
		`, professionID, "Test Get By Name Go Developer #4", "go developer test 4")
		require.NoError(t, err)

		// Тест
		profession, err := storage.GetProfessionByName(testCtx, "Test Get By Name Go Developer #4")

		// Assert
		require.NoError(t, err)
		require.Equal(t, professionID, profession.ID)
		require.Equal(t, "Test Get By Name Go Developer #4", profession.Name)
		require.True(t, profession.IsActive)
	})

	t.Run("GetProfessionByName_NotFound", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Тест (несуществующее имя)
		profession, err := storage.GetProfessionByName(testCtx, "Nonexistent Profession")

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionNotFound)
		require.Empty(t, profession)
	})

	t.Run("AddProfession_Success", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Тест - добавляем профессию с уникальным именем
		profession := domain.Profession{
			Name:         "Test Add Go Developer #5",
			VacancyQuery: "go developer test 5",
			IsActive:     true,
		}

		professionID, err := storage.AddProfession(testCtx, profession)

		// Assert
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, professionID)

		// Проверяем что профессия действительно создана
		created, err := storage.GetProfessionByID(testCtx, professionID)
		require.NoError(t, err)
		require.Equal(t, profession.Name, created.Name)
		require.Equal(t, profession.VacancyQuery, created.VacancyQuery)
	})

	t.Run("AddProfession_AlreadyExists", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Создание первой профессии с уникальным именем
		profession := domain.Profession{
			Name:         "Test Add Duplicate Go Developer #6",
			VacancyQuery: "go developer test 6",
			IsActive:     true,
		}

		insertProfession(t, storage, profession)

		// Тест - попытка добавить такую же профессию (по имени)
		professionID, err := storage.AddProfession(testCtx, profession)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionAlreadyExists)
		require.Equal(t, uuid.Nil, professionID)
	})

	t.Run("UpdateProfession_Success", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Создание тестовой профессии с уникальным именем
		profession := domain.Profession{
			Name:         "Test Update Go Developer #7",
			VacancyQuery: "go developer test 7",
			IsActive:     true,
		}
		professionID := insertProfession(t, storage, profession)

		// Тест - обновление
		updatedProfession := domain.Profession{
			ID:           professionID,
			Name:         "Test Updated Senior Go Developer #7",
			VacancyQuery: "senior go developer test 7",
			IsActive:     false,
		}

		err := storage.UpdateProfession(testCtx, updatedProfession)

		// Assert
		require.NoError(t, err)

		// Проверяем что профессия обновилась
		updated, err := storage.GetProfessionByID(testCtx, professionID)
		require.NoError(t, err)
		require.Equal(t, "Test Updated Senior Go Developer #7", updated.Name)
		require.Equal(t, "senior go developer test 7", updated.VacancyQuery)
		require.False(t, updated.IsActive)
	})

	t.Run("UpdateProfession_NotFound", func(t *testing.T) {
		cleanProfessionTable(t, storage)

		// Тест - обновление несуществующей профессии
		nonExistentID := uuid.New()
		profession := domain.Profession{
			ID:           nonExistentID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		}

		err := storage.UpdateProfession(testCtx, profession)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionNotFound)
	})

	t.Run("UpdateProfession_AlreadyExists", func(t *testing.T) {
		cleanProfessionTable(t, storage)

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

		insertProfession(t, storage, profession1)
		id2 := insertProfession(t, storage, profession2)

		// Пытаемся обновить profession2 именем profession1 (должен быть unique violation)
		profession2.ID = id2
		profession2.Name = profession1.Name

		err := storage.UpdateProfession(testCtx, profession2)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrProfessionAlreadyExists)
	})
}
