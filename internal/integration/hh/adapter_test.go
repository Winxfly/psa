package hh

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProfessionFetcher — ручная реализация professionFetcher для тестов
type fakeProfessionFetcher struct {
	fn func(ctx context.Context, query, area string) (professionData, error)
}

func (f *fakeProfessionFetcher) fetchDataProfession(ctx context.Context, query, area string) (professionData, error) {
	return f.fn(ctx, query, area)
}

// skills — хелпер для создания списка навыков
func skills(names ...string) []struct {
	Name string `json:"name"`
} {
	res := make([]struct {
		Name string `json:"name"`
	}, 0, len(names))
	for _, n := range names {
		res = append(res, struct {
			Name string `json:"name"`
		}{Name: n})
	}
	return res
}

func TestAdapter_FetchDataProfession_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies: []vacancyResponse{
					{
						Description: "We need a Go developer",
						KeySkills:   skills("Golang", "  Python  ", "SQL"),
					},
					{
						Description: "Another vacancy",
						KeySkills:   skills("golang", "Docker"),
					},
				},
				TotalFound: 150,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 150, total)
	assert.Len(t, result, 2)

	// Проверяем первую вакансию
	assert.Equal(t, "We need a Go developer", result[0].Description)
	assert.Equal(t, []string{"golang", "python", "sql"}, result[0].Skills)

	// Проверяем вторую вакансию
	assert.Equal(t, "Another vacancy", result[1].Description)
	assert.Equal(t, []string{"golang", "docker"}, result[1].Skills)
}

func TestAdapter_FetchDataProfession_EmptySkills(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies: []vacancyResponse{
					{
						Description: "Vacancy without skills",
						KeySkills:   skills(),
					},
				},
				TotalFound: 50,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 50, total)
	assert.Len(t, result, 1)
	assert.Empty(t, result[0].Skills)
}

func TestAdapter_FetchDataProfession_EmptyVacancies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies:  []vacancyResponse{},
				TotalFound: 0,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, result)
}

func TestAdapter_FetchDataProfession_SkillFiltering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies: []vacancyResponse{
					{
						Description: "Test",
						KeySkills:   skills("golang", "   ", "!!!", "python", "C++", "  java  "),
					},
				},
				TotalFound: 10,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Len(t, result, 1)
	// Ожидаем: golang, python, C++, java (4 навыка)
	// "   " — отфильтрован (пустой после trim)
	// "!!!" — отфильтрован (нет букв/цифр)
	assert.Equal(t, []string{"golang", "python", "c++", "java"}, result[0].Skills)
}

func TestAdapter_FetchDataProfession_ClientError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{}, assert.AnError
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, result)
}

func TestAdapter_FetchDataProfession_Normalization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies: []vacancyResponse{
					{
						Description: "Test",
						KeySkills:   skills("GOLANG", "Python", "  SQL  "),
					},
				},
				TotalFound: 5,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, result, 1)
	// Проверяем что все навыки в нижнем регистре и без пробелов
	assert.Equal(t, []string{"golang", "python", "sql"}, result[0].Skills)
}

func TestAdapter_FetchDataProfession_DuplicateSkills(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies: []vacancyResponse{
					{
						Description: "Test",
						KeySkills:   skills("golang", "GOLANG", "Go"),
					},
				},
				TotalFound: 5,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, result, 1)
	// Дубликаты сохраняются (это текущее поведение)
	assert.Equal(t, []string{"golang", "golang", "go"}, result[0].Skills)
}

func TestAdapter_FetchDataProfession_AllSkillsFiltered(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := "go developer"
	area := "113"

	// Arrange
	fakeClient := &fakeProfessionFetcher{
		fn: func(ctx context.Context, query, area string) (professionData, error) {
			return professionData{
				Vacancies: []vacancyResponse{
					{
						Description: "Test",
						KeySkills:   skills("   ", "!!!", "---", "***"),
					},
				},
				TotalFound: 3,
			}, nil
		},
	}

	adapter := NewAdapterWithClient(fakeClient)

	// Act
	result, total, err := adapter.FetchDataProfession(ctx, query, area)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, result, 1)
	assert.Empty(t, result[0].Skills)
}
