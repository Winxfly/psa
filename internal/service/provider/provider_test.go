package provider

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/service/provider/mocks"
)

// testDeps содержит зависимости для тестирования Provider
type testDeps struct {
	professionProvider *mocks.MockProfessionProvider
	sessionProvider    *mocks.MockSessionProvider
	statProvider       *mocks.MockStatProvider
	skillsProvider     *mocks.MockSkillsProvider
	cache              *mocks.MockCacheProvider
	dailyStatProvider  *mocks.MockDailyStatProvider
}

func newDeps(t *testing.T) testDeps {
	t.Helper()
	return testDeps{
		professionProvider: mocks.NewMockProfessionProvider(t),
		sessionProvider:    mocks.NewMockSessionProvider(t),
		statProvider:       mocks.NewMockStatProvider(t),
		skillsProvider:     mocks.NewMockSkillsProvider(t),
		cache:              mocks.NewMockCacheProvider(t),
		dailyStatProvider:  mocks.NewMockDailyStatProvider(t),
	}
}

func (d testDeps) provider() *Provider {
	return New(
		d.professionProvider,
		d.sessionProvider,
		d.statProvider,
		d.skillsProvider,
		d.cache,
		d.dailyStatProvider,
	)
}

// ==================== ActiveProfessions ====================

func TestProvider_ActiveProfessions_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)
	// cache = nil — избегаем асинхронных вызовов

	professions := []domain.Profession{
		{
			ID:           uuid.New(),
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
		{
			ID:           uuid.New(),
			Name:         "Python Developer",
			VacancyQuery: "python developer",
			IsActive:     true,
		},
	}

	expectedActiveProfessions := []domain.ActiveProfession{
		{
			ID:           professions[0].ID,
			Name:         professions[0].Name,
			VacancyQuery: professions[0].VacancyQuery,
		},
		{
			ID:           professions[1].ID,
			Name:         professions[1].Name,
			VacancyQuery: professions[1].VacancyQuery,
		},
	}

	professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache = nil для unit теста
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ActiveProfessions(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, expectedActiveProfessions[0].ID, result[0].ID)
	assert.Equal(t, expectedActiveProfessions[0].Name, result[0].Name)
	assert.Equal(t, expectedActiveProfessions[1].ID, result[1].ID)
	assert.Equal(t, expectedActiveProfessions[1].Name, result[1].Name)
}

func TestProvider_ActiveProfessions_CacheHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	cachedProfessions := []domain.ActiveProfession{
		{
			ID:           uuid.New(),
			Name:         "Cached Go Developer",
			VacancyQuery: "cached go",
		},
	}

	deps.cache.EXPECT().GetProfessionsList(ctx).Return(cachedProfessions, nil)

	providerService := deps.provider()

	// Act
	result, err := providerService.ActiveProfessions(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Cached Go Developer", result[0].Name)
	deps.professionProvider.AssertNotCalled(t, "GetActiveProfessions")
}

func TestProvider_ActiveProfessions_CacheMiss(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)
	// cache = nil — избегаем асинхронных вызовов

	professions := []domain.Profession{
		{
			ID:           uuid.New(),
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	expectedActiveProfessions := []domain.ActiveProfession{
		{
			ID:           professions[0].ID,
			Name:         professions[0].Name,
			VacancyQuery: professions[0].VacancyQuery,
		},
	}

	professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache = nil для unit теста
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ActiveProfessions(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, expectedActiveProfessions[0].ID, result[0].ID)
}

func TestProvider_ActiveProfessions_CacheError_Fallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professions := []domain.Profession{
		{
			ID:           uuid.New(),
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	expectedActiveProfessions := []domain.ActiveProfession{
		{
			ID:           professions[0].ID,
			Name:         professions[0].Name,
			VacancyQuery: professions[0].VacancyQuery,
		},
	}

	// Cache возвращает ошибку — должен быть fallback в БД
	deps.cache.EXPECT().GetProfessionsList(ctx).Return(nil, assert.AnError)
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.cache.EXPECT().SaveProfessionsList(mock.Anything, mock.Anything).Return(nil)

	providerService := deps.provider()

	// Act
	result, err := providerService.ActiveProfessions(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, expectedActiveProfessions[0].ID, result[0].ID)
	// Даём время на асинхронное сохранение в кэш
	time.Sleep(50 * time.Millisecond)
}

func TestProvider_ActiveProfessions_GetActiveProfessionsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)

	dbError := assert.AnError
	professionProvider.EXPECT().GetActiveProfessions(ctx).Return(nil, dbError)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache = nil
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ActiveProfessions(ctx)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
}

func TestProvider_ActiveProfessions_NilCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)
	// cache = nil

	professions := []domain.Profession{
		{
			ID:           uuid.New(),
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache == nil
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ActiveProfessions(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 1)
}

// ==================== AllProfessions ====================

func TestProvider_AllProfessions_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professions := []domain.Profession{
		{
			ID:           uuid.New(),
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
		{
			ID:           uuid.New(),
			Name:         "Python Developer",
			VacancyQuery: "python developer",
			IsActive:     false,
		},
	}

	deps.professionProvider.EXPECT().GetAllProfessions(ctx).Return(professions, nil)

	providerService := deps.provider()

	// Act
	result, err := providerService.AllProfessions(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Go Developer", result[0].Name)
	assert.Equal(t, "Python Developer", result[1].Name)
}

func TestProvider_AllProfessions_GetAllProfessionsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	dbError := assert.AnError
	deps.professionProvider.EXPECT().GetAllProfessions(ctx).Return(nil, dbError)

	providerService := deps.provider()

	// Act
	result, err := providerService.AllProfessions(ctx)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
}

// ==================== ProfessionByID ====================

func TestProvider_ProfessionByID_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	profession := domain.Profession{
		ID:           professionID,
		Name:         "Go Developer",
		VacancyQuery: "go developer",
		IsActive:     true,
	}

	deps.professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(profession, nil)

	providerService := deps.provider()

	// Act
	result, err := providerService.ProfessionByID(ctx, professionID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, professionID, result.ID)
	assert.Equal(t, "Go Developer", result.Name)
}

func TestProvider_ProfessionByID_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	deps.professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(domain.Profession{}, domain.ErrProfessionNotFound)

	providerService := deps.provider()

	// Act
	result, err := providerService.ProfessionByID(ctx, professionID)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrProfessionNotFound)
}

func TestProvider_ProfessionByID_DatabaseError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	dbError := assert.AnError
	deps.professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(domain.Profession{}, dbError)

	providerService := deps.provider()

	// Act
	result, err := providerService.ProfessionByID(ctx, professionID)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
}

// ==================== CreateProfession ====================

func TestProvider_CreateProfession_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	newProfession := domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "go developer",
	}
	newProfessionID := uuid.New()

	deps.professionProvider.EXPECT().AddProfession(ctx, mock.MatchedBy(func(p domain.Profession) bool {
		return p.Name == "Go Developer" && p.VacancyQuery == "go developer" && p.IsActive
	})).Return(newProfessionID, nil)

	providerService := deps.provider()

	// Act
	resultID, err := providerService.CreateProfession(ctx, newProfession)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, newProfessionID, resultID)
}

func TestProvider_CreateProfession_EmptyName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	newProfession := domain.Profession{
		Name:         "",
		VacancyQuery: "go developer",
	}

	providerService := deps.provider()

	// Act
	resultID, err := providerService.CreateProfession(ctx, newProfession)

	// Assert
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, resultID)
	assert.ErrorIs(t, err, domain.ErrInvalidProfessionName)
	deps.professionProvider.AssertNotCalled(t, "AddProfession")
}

func TestProvider_CreateProfession_WhitespaceOnlyName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	// Edge case: имя содержит только пробелы
	newProfession := domain.Profession{
		Name:         "   ",
		VacancyQuery: "go developer",
	}

	providerService := deps.provider()

	// Act
	resultID, err := providerService.CreateProfession(ctx, newProfession)

	// Assert
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, resultID)
	assert.ErrorIs(t, err, domain.ErrInvalidProfessionName)
	deps.professionProvider.AssertNotCalled(t, "AddProfession")
}

func TestProvider_CreateProfession_WhitespaceOnlyQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	// Edge case: query содержит только пробелы
	newProfession := domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "   ",
	}

	providerService := deps.provider()

	// Act
	resultID, err := providerService.CreateProfession(ctx, newProfession)

	// Assert
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, resultID)
	assert.ErrorIs(t, err, domain.ErrInvalidProfessionQuery)
	deps.professionProvider.AssertNotCalled(t, "AddProfession")
}

func TestProvider_CreateProfession_EmptyQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	newProfession := domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "",
	}

	providerService := deps.provider()

	// Act
	resultID, err := providerService.CreateProfession(ctx, newProfession)

	// Assert
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, resultID)
	assert.ErrorIs(t, err, domain.ErrInvalidProfessionQuery)
	deps.professionProvider.AssertNotCalled(t, "AddProfession")
}

func TestProvider_CreateProfession_AlreadyExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	newProfession := domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "go developer",
	}

	deps.professionProvider.EXPECT().AddProfession(ctx, mock.Anything).Return(uuid.Nil, domain.ErrProfessionAlreadyExists)

	providerService := deps.provider()

	// Act
	resultID, err := providerService.CreateProfession(ctx, newProfession)

	// Assert
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, resultID)
	assert.ErrorIs(t, err, domain.ErrProfessionAlreadyExists)
}

// ==================== ChangeProfession ====================

func TestProvider_ChangeProfession_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	profession := domain.Profession{
		ID:           uuid.New(),
		Name:         "Updated Go Developer",
		VacancyQuery: "updated go developer",
		IsActive:     true,
	}

	deps.professionProvider.EXPECT().UpdateProfession(ctx, profession).Return(nil)

	providerService := deps.provider()

	// Act
	err := providerService.ChangeProfession(ctx, profession)

	// Assert
	require.NoError(t, err)
}

func TestProvider_ChangeProfession_NilID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	profession := domain.Profession{
		ID:           uuid.Nil,
		Name:         "Go Developer",
		VacancyQuery: "go developer",
	}

	providerService := deps.provider()

	// Act
	err := providerService.ChangeProfession(ctx, profession)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidProfessionID)
	deps.professionProvider.AssertNotCalled(t, "UpdateProfession")
}

func TestProvider_ChangeProfession_EmptyName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	profession := domain.Profession{
		ID:           uuid.New(),
		Name:         "",
		VacancyQuery: "go developer",
	}

	providerService := deps.provider()

	// Act
	err := providerService.ChangeProfession(ctx, profession)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidProfessionName)
	deps.professionProvider.AssertNotCalled(t, "UpdateProfession")
}

func TestProvider_ChangeProfession_AlreadyExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	profession := domain.Profession{
		ID:           uuid.New(),
		Name:         "Go Developer",
		VacancyQuery: "go developer",
	}

	deps.professionProvider.EXPECT().UpdateProfession(ctx, profession).Return(domain.ErrProfessionAlreadyExists)

	providerService := deps.provider()

	// Act
	err := providerService.ChangeProfession(ctx, profession)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrProfessionAlreadyExists)
}

// ==================== ProfessionSkills ====================

func TestProvider_ProfessionSkills_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)
	// cache = nil — избегаем асинхронных вызовов

	professionID := uuid.New()
	scrapingID := uuid.New()

	profession := domain.Profession{
		ID:           professionID,
		Name:         "Go Developer",
		VacancyQuery: "go developer",
		IsActive:     true,
	}

	latestScraping := domain.Scraping{
		ID:        scrapingID,
		ScrapedAt: time.Now(),
	}

	stat := domain.Stat{
		VacancyCount: 100,
	}

	formalSkills := []domain.Skill{
		{Skill: "go", Count: 50},
		{Skill: "python", Count: 30},
	}

	extractedSkills := []domain.Skill{
		{Skill: "gin", Count: 20},
		{Skill: "docker", Count: 15},
	}

	professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(profession, nil)
	sessionProvider.EXPECT().GetLatestScraping(ctx).Return(latestScraping, nil)
	statProvider.EXPECT().GetLatestStatByProfessionID(ctx, professionID).Return(stat, nil)
	skillsProvider.EXPECT().GetFormalSkillsByProfessionAndDate(ctx, professionID, scrapingID).Return(formalSkills, nil)
	skillsProvider.EXPECT().GetExtractedSkillsByProfessionAndDate(ctx, professionID, scrapingID).Return(extractedSkills, nil)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache = nil для unit теста
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ProfessionSkills(ctx, professionID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, professionID, result.ProfessionID)
	assert.Equal(t, "Go Developer", result.ProfessionName)
	assert.Equal(t, int32(100), result.VacancyCount)
	require.Len(t, result.FormalSkills, 2)
	assert.Equal(t, "go", result.FormalSkills[0].Skill)
	assert.Equal(t, int32(50), result.FormalSkills[0].Count)
	require.Len(t, result.ExtractedSkills, 2)
}

func TestProvider_ProfessionSkills_CacheHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()

	cachedData := &domain.ProfessionDetail{
		ProfessionID:   professionID,
		ProfessionName: "Cached Go Developer",
		VacancyCount:   50,
	}

	deps.cache.EXPECT().GetProfessionData(ctx, professionID).Return(cachedData, nil)

	providerService := deps.provider()

	// Act
	result, err := providerService.ProfessionSkills(ctx, professionID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Cached Go Developer", result.ProfessionName)
	deps.professionProvider.AssertNotCalled(t, "GetProfessionByID")
	deps.sessionProvider.AssertNotCalled(t, "GetLatestScraping")
	deps.statProvider.AssertNotCalled(t, "GetLatestStatByProfessionID")
	deps.skillsProvider.AssertNotCalled(t, "GetFormalSkillsByProfessionAndDate")
	deps.skillsProvider.AssertNotCalled(t, "GetExtractedSkillsByProfessionAndDate")
}

func TestProvider_ProfessionSkills_ProfessionNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)

	professionID := uuid.New()
	professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(domain.Profession{}, domain.ErrProfessionNotFound)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache = nil
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ProfessionSkills(ctx, professionID)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrProfessionNotFound)
}

// ==================== ProfessionTrend ====================

func TestProvider_ProfessionTrend_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)
	// cache = nil — избегаем асинхронных вызовов

	professionID := uuid.New()

	profession := domain.Profession{
		ID:           professionID,
		Name:         "Go Developer",
		VacancyQuery: "go developer",
	}

	statPoints := []domain.StatDailyPoint{
		{Date: time.Now().AddDate(0, 0, -2), VacancyCount: 80},
		{Date: time.Now().AddDate(0, 0, -1), VacancyCount: 90},
		{Date: time.Now(), VacancyCount: 100},
	}

	professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(profession, nil)
	dailyStatProvider.EXPECT().GetStatDailyByProfessionID(ctx, professionID).Return(statPoints, nil)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil, // cache = nil для unit теста
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ProfessionTrend(ctx, professionID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, professionID, result.ProfessionID)
	assert.Equal(t, "Go Developer", result.ProfessionName)
	require.Len(t, result.Data, 3)
}

func TestProvider_ProfessionTrend_CacheHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()

	cachedTrend := &domain.ProfessionTrend{
		ProfessionID:   professionID,
		ProfessionName: "Cached Go Developer",
		Data:           []domain.StatDailyPoint{{Date: time.Now(), VacancyCount: 50}},
	}

	deps.cache.EXPECT().GetProfessionTrend(ctx, professionID).Return(cachedTrend, nil)

	providerService := deps.provider()

	// Act
	result, err := providerService.ProfessionTrend(ctx, professionID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Cached Go Developer", result.ProfessionName)
	deps.professionProvider.AssertNotCalled(t, "GetProfessionByID")
	deps.dailyStatProvider.AssertNotCalled(t, "GetStatDailyByProfessionID")
}

func TestProvider_ProfessionTrend_ProfessionNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)

	professionID := uuid.New()
	professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(domain.Profession{}, domain.ErrProfessionNotFound)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil,
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ProfessionTrend(ctx, professionID)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrProfessionNotFound)
}

func TestProvider_ProfessionTrend_GetStatDailyError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)

	professionID := uuid.New()
	profession := domain.Profession{
		ID:           professionID,
		Name:         "Go Developer",
		VacancyQuery: "go developer",
	}

	dbError := assert.AnError
	professionProvider.EXPECT().GetProfessionByID(ctx, professionID).Return(profession, nil)
	dailyStatProvider.EXPECT().GetStatDailyByProfessionID(ctx, professionID).Return(nil, dbError)

	providerService := New(
		professionProvider,
		sessionProvider,
		statProvider,
		skillsProvider,
		nil,
		dailyStatProvider,
	)

	// Act
	result, err := providerService.ProfessionTrend(ctx, professionID)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
}
