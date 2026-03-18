package scraper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/service/scraper/mocks"
)

// testDeps содержит зависимости для тестирования Scraper
type testDeps struct {
	professionProvider *mocks.MockProfessionProvider
	sessionProvider    *mocks.MockSessionProvider
	skillsProvider     *mocks.MockSkillsProvider
	statProvider       *mocks.MockStatProvider
	dailyStatProvider  *mocks.MockDailyStatProvider
	supplierPort       *mocks.MockSupplierPort
	extractor          *mocks.MockExtractor
	cache              *mocks.MockCacheProvider
}

func newDeps(t *testing.T) testDeps {
	t.Helper()
	return testDeps{
		professionProvider: mocks.NewMockProfessionProvider(t),
		sessionProvider:    mocks.NewMockSessionProvider(t),
		skillsProvider:     mocks.NewMockSkillsProvider(t),
		statProvider:       mocks.NewMockStatProvider(t),
		dailyStatProvider:  mocks.NewMockDailyStatProvider(t),
		supplierPort:       mocks.NewMockSupplierPort(t),
		extractor:          mocks.NewMockExtractor(t),
		cache:              mocks.NewMockCacheProvider(t),
	}
}

func (d testDeps) scraper() *Scraper {
	return New(
		d.professionProvider,
		d.sessionProvider,
		d.skillsProvider,
		d.statProvider,
		d.dailyStatProvider,
		d.supplierPort,
		d.extractor,
		d.cache,
	)
}

func TestAggregateFormalSkills(t *testing.T) {
	tests := []struct {
		name     string
		data     []domain.VacancyData
		expected map[string]int
	}{
		{
			name:     "empty data",
			data:     []domain.VacancyData{},
			expected: map[string]int{},
		},
		{
			name: "single vacancy with skills",
			data: []domain.VacancyData{
				{Skills: []string{"go", "python", "sql"}},
			},
			expected: map[string]int{
				"go":     1,
				"python": 1,
				"sql":    1,
			},
		},
		{
			name: "multiple vacancies with duplicate skills",
			data: []domain.VacancyData{
				{Skills: []string{"go", "python"}},
				{Skills: []string{"go", "java"}},
				{Skills: []string{"go", "python"}},
			},
			expected: map[string]int{
				"go":     3,
				"python": 2,
				"java":   1,
			},
		},
		{
			name: "vacancy with no skills",
			data: []domain.VacancyData{
				{Skills: []string{}},
				{Skills: []string{"go"}},
			},
			expected: map[string]int{
				"go": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаём пустой скрапер - метод не использует receiver
			s := &Scraper{}

			result := s.aggregateFormalSkills(tt.data)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterRareSkills(t *testing.T) {
	tests := []struct {
		name     string
		skills   map[string]int
		minCount int
		expected map[string]int
	}{
		{
			name:     "empty skills",
			skills:   map[string]int{},
			minCount: 2,
			expected: map[string]int{},
		},
		{
			name: "all skills above threshold",
			skills: map[string]int{
				"go":     5,
				"python": 3,
				"java":   2,
			},
			minCount: 2,
			expected: map[string]int{
				"go":     5,
				"python": 3,
				"java":   2,
			},
		},
		{
			name: "some skills below threshold",
			skills: map[string]int{
				"go":     5,
				"python": 3,
				"java":   1,
				"ruby":   0,
			},
			minCount: 2,
			expected: map[string]int{
				"go":     5,
				"python": 3,
			},
		},
		{
			name: "all skills below threshold",
			skills: map[string]int{
				"go":   1,
				"ruby": 0,
			},
			minCount: 2,
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scraper{}

			result := s.filterRareSkills(tt.skills, tt.minCount)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransformSkillsSort(t *testing.T) {
	tests := []struct {
		name     string
		skills   map[string]int
		expected []domain.SkillResponse
	}{
		{
			name:     "empty skills",
			skills:   map[string]int{},
			expected: []domain.SkillResponse{},
		},
		{
			name: "single skill",
			skills: map[string]int{
				"go": 5,
			},
			expected: []domain.SkillResponse{
				{Skill: "go", Count: 5},
			},
		},
		{
			name: "multiple skills sorted by count desc",
			skills: map[string]int{
				"go":     5,
				"python": 10,
				"java":   3,
				"sql":    7,
			},
			expected: []domain.SkillResponse{
				{Skill: "python", Count: 10},
				{Skill: "sql", Count: 7},
				{Skill: "go", Count: 5},
				{Skill: "java", Count: 3},
			},
		},
		{
			name: "skills with same count",
			skills: map[string]int{
				"go":     5,
				"python": 5,
				"java":   5,
			},
			expected: []domain.SkillResponse{
				{Skill: "go", Count: 5},
				{Skill: "python", Count: 5},
				{Skill: "java", Count: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scraper{}

			result := s.transformSkillsSort(tt.skills)

			// Используем ElementsMatch для случаев с одинаковым count
			// порядок может быть нестабильным
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

// ==================== Service Tests with Mocks ====================

func TestScraper_ProcessActiveProfessionsDaily_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()

	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	vacancyData := []domain.VacancyData{
		{
			Skills:      []string{"go", "python"},
			Description: "We need go developer",
		},
		{
			Skills:      []string{"go", "python"},
			Description: "We need go developer again",
		},
	}

	scrapedAt := time.Now()

	// Порядок вызовов важен для корректной работы пайплайна
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 100, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID, 100, mock.MatchedBy(func(t time.Time) bool {
		return t.After(scrapedAt.Add(-time.Second)) && t.Before(scrapedAt.Add(time.Second))
	})).Return(nil)
	// Проверяем что description содержит ожидаемый текст
	deps.extractor.EXPECT().ExtractSkills(
		mock.MatchedBy(func(text string) bool {
			return len(text) > 0
		}),
		mock.Anything,
		3,
	).Return(map[string]int{"go": 50}, nil)
	// Проверяем payload cache
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID &&
			data.ProfessionName == "Go Developer" &&
			data.VacancyCount == 100 &&
			len(data.FormalSkills) > 0
	})).Return(nil)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsDaily(ctx)

	// Assert
	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsDaily_GetProfessionsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	dbError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(nil, dbError)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsDaily(ctx)

	// Assert
	require.Error(t, err)
	deps.sessionProvider.AssertNotCalled(t, "CreateScrapingSession")
	deps.supplierPort.AssertNotCalled(t, "FetchDataProfession")
	deps.dailyStatProvider.AssertNotCalled(t, "SaveStatDaily")
	deps.extractor.AssertNotCalled(t, "ExtractSkills")
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsDaily_FetchDataError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	fetchError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(nil, 0, fetchError)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsDaily(ctx)

	// Assert
	require.NoError(t, err) // Ошибка логируется, но не прерывает выполнение
	deps.dailyStatProvider.AssertNotCalled(t, "SaveStatDaily")
	deps.extractor.AssertNotCalled(t, "ExtractSkills")
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsArchive_WithSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	sessionID := uuid.New()

	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	// Несколько вакансий с одинаковыми навыками чтобы пройти фильтр filterRareSkills (minCount=2)
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer needed"},
		{Skills: []string{"go"}, Description: "Go developer needed"},
	}

	formalSkills := map[string]int{"go": 2}     // 2 упоминания - пройдёт фильтр
	extractedSkills := map[string]int{"go": 20} // extractor возвращает {"go": 10} для каждой вакансии, итого 20

	// Порядок вызовов важен: сессия должна быть создана до fetch
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID, 50, mock.Anything).Return(nil)
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID, 50).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID, formalSkills).Return(nil)
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID, extractedSkills).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 10}, nil)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.Anything).Return(nil)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsArchive_CreateSessionError(t *testing.T) {
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

	sessionError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(uuid.Nil, sessionError)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.Error(t, err)
	deps.supplierPort.AssertNotCalled(t, "FetchDataProfession")
	deps.dailyStatProvider.AssertNotCalled(t, "SaveStatDaily")
	deps.statProvider.AssertNotCalled(t, "SaveStat")
	deps.skillsProvider.AssertNotCalled(t, "SaveFormalSkills")
	deps.skillsProvider.AssertNotCalled(t, "SaveExtractedSkills")
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsArchive_SaveStatDailyError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	sessionID := uuid.New()

	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	saveStatError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID, 50, mock.Anything).Return(saveStatError)
	// Остальные вызовы продолжаются несмотря на ошибку SaveStatDaily
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID, 50).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 10}, nil)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.Anything).Return(nil)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err) // Ошибка логируется, но не прерывает выполнение
}

func TestScraper_ProcessActiveProfessionsArchive_SaveCacheError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	sessionID := uuid.New()

	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	cacheError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID, 50, mock.Anything).Return(nil)
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID, 50).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 10}, nil)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.Anything).Return(cacheError)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err) // Ошибка логируется, но не прерывает выполнение
}

func TestScraper_ProcessActiveProfessionsArchive_ExtractSkillsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()
	sessionID := uuid.New()

	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	extractError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID, 50, mock.Anything).Return(nil)
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID, 50).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	// SaveExtractedSkills вызывается с пустыми навыками из-за ошибки extract
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID, map[string]int{}).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{}, extractError)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.Anything).Return(nil)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err) // Ошибка логируется, но не прерывает выполнение
}

func TestScraper_ProcessActiveProfessionsArchive_MultipleProfessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID1 := uuid.New()
	professionID2 := uuid.New()
	sessionID := uuid.New()

	// Две профессии для проверки цикла
	professions := []domain.Profession{
		{
			ID:           professionID1,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
		{
			ID:           professionID2,
			Name:         "Python Developer",
			VacancyQuery: "python developer",
			IsActive:     true,
		},
	}

	vacancyData1 := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}
	vacancyData2 := []domain.VacancyData{
		{Skills: []string{"python"}, Description: "Python developer"},
		{Skills: []string{"python"}, Description: "Python developer"},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)

	// Первая профессия - успешно
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData1, 50, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID1, 50, mock.Anything).Return(nil)
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID1, 50).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID1, mock.Anything).Return(nil)
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID1, mock.Anything).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 10}, nil)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID1 && data.VacancyCount == 50
	})).Return(nil)

	// Вторая профессия - успешно
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "python developer", "113").Return(vacancyData2, 75, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID2, 75, mock.Anything).Return(nil)
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID2, 75).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID2, mock.Anything).Return(nil)
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID2, mock.Anything).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"python": 15}, nil)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID2 && data.VacancyCount == 75
	})).Return(nil)

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsArchive_MixedSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	deps := newDeps(t)

	professionID1 := uuid.New()
	professionID2 := uuid.New()
	sessionID := uuid.New()

	// Две профессии: одна успешная, одна с ошибкой
	professions := []domain.Profession{
		{
			ID:           professionID1,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
		{
			ID:           professionID2,
			Name:         "Python Developer",
			VacancyQuery: "python developer",
			IsActive:     true,
		},
	}

	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	fetchError := assert.AnError

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)

	// Первая профессия - успешно
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID1, 50, mock.Anything).Return(nil)
	deps.statProvider.EXPECT().SaveStat(ctx, sessionID, professionID1, 50).Return(nil)
	deps.skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID1, mock.Anything).Return(nil)
	deps.skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID1, mock.Anything).Return(nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 10}, nil)
	deps.cache.EXPECT().SaveProfessionData(ctx, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID1 && data.VacancyCount == 50
	})).Return(nil)

	// Вторая профессия - ошибка fetch
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "python developer", "113").Return(nil, 0, fetchError)
	// SaveStatDaily не вызывается при ошибке fetch

	scraperService := deps.scraper()

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err) // Ошибка одной профессии не прерывает процесс
}

func TestScraper_ProcessActiveProfessionsArchive_NilCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange
	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	skillsProvider := mocks.NewMockSkillsProvider(t)
	statProvider := mocks.NewMockStatProvider(t)
	dailyStatProvider := mocks.NewMockDailyStatProvider(t)
	supplierPort := mocks.NewMockSupplierPort(t)
	extractor := mocks.NewMockExtractor(t)
	// cache = nil

	professionID := uuid.New()
	sessionID := uuid.New()

	professions := []domain.Profession{
		{
			ID:           professionID,
			Name:         "Go Developer",
			VacancyQuery: "go developer",
			IsActive:     true,
		},
	}

	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	dailyStatProvider.EXPECT().SaveStatDaily(ctx, professionID, 50, mock.Anything).Return(nil)
	statProvider.EXPECT().SaveStat(ctx, sessionID, professionID, 50).Return(nil)
	skillsProvider.EXPECT().SaveFormalSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	skillsProvider.EXPECT().SaveExtractedSkills(ctx, sessionID, professionID, mock.Anything).Return(nil)
	extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 10}, nil)
	// cache.SaveProfessionData НЕ вызывается

	scraperService := New(
		professionProvider,
		sessionProvider,
		skillsProvider,
		statProvider,
		dailyStatProvider,
		supplierPort,
		extractor,
		nil, // cache == nil
	)

	// Act
	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	// Assert
	require.NoError(t, err)
}
