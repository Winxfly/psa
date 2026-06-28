package scraper

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/service/scraper/mocks"
)

type testDeps struct {
	professionProvider *mocks.MockProfessionProvider
	sessionProvider    *mocks.MockSessionProvider
	corpusProvider     *mocks.MockSkillCorpusProvider
	resultSaver        *mocks.MockScrapeResultSaver
	supplierPort       *mocks.MockSupplierPort
	extractor          *mocks.MockExtractor
	cache              *mocks.MockCacheProvider
}

func newDeps(t *testing.T) testDeps {
	t.Helper()

	return testDeps{
		professionProvider: mocks.NewMockProfessionProvider(t),
		sessionProvider:    mocks.NewMockSessionProvider(t),
		corpusProvider:     mocks.NewMockSkillCorpusProvider(t),
		resultSaver:        mocks.NewMockScrapeResultSaver(t),
		supplierPort:       mocks.NewMockSupplierPort(t),
		extractor:          mocks.NewMockExtractor(t),
		cache:              mocks.NewMockCacheProvider(t),
	}
}

func (d testDeps) scraper() *Scraper {
	return New(
		d.professionProvider,
		d.sessionProvider,
		d.corpusProvider,
		d.resultSaver,
		d.supplierPort,
		d.extractor,
		d.cache,
	)
}

func matchScrapeResult(professionID uuid.UUID, vacancyCount int,
	formalSkills map[string]int, extractedSkills map[string]int) interface{} {
	return mock.MatchedBy(func(result domain.ProfessionScrapeResult) bool {
		return result.ProfessionID == professionID &&
			result.VacancyCount == vacancyCount &&
			reflect.DeepEqual(result.FormalSkills, formalSkills) &&
			reflect.DeepEqual(result.ExtractedSkills, extractedSkills)
	})
}

func matchScrapedAt() interface{} {
	return mock.MatchedBy(func(scrapedAt time.Time) bool {
		return !scrapedAt.IsZero()
	})
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
			s := &Scraper{}

			result := s.aggregateFormalSkills(tt.data)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterSkillsByMinCount(t *testing.T) {
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
			result := filterSkillsByMinCount(tt.skills, tt.minCount)

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
			name: "skills with same count sorted by name",
			skills: map[string]int{
				"go":     5,
				"python": 5,
				"java":   5,
			},
			expected: []domain.SkillResponse{
				{Skill: "go", Count: 5},
				{Skill: "java", Count: 5},
				{Skill: "python", Count: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scraper{}

			result := s.transformSkillsSort(tt.skills)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildGlobalWhitelist(t *testing.T) {
	professionID := uuid.New()
	aboveThresholdCount := whitelistSkillMinCount
	belowThresholdCount := whitelistSkillMinCount - 1

	storedCorpus := map[uuid.UUID]map[string]int{
		professionID: {
			"go":         aboveThresholdCount,
			"docker":     belowThresholdCount,
			"разработка": aboveThresholdCount,
		},
	}
	items := []professionScrapeData{
		{
			filteredFormalSkills: map[string]int{
				"go":         belowThresholdCount,
				"postgresql": aboveThresholdCount,
				"с":          aboveThresholdCount,
			},
		},
	}

	result := buildGlobalWhitelist(storedCorpus, items)

	assert.Equal(t, map[string]int{
		"go":         aboveThresholdCount + belowThresholdCount,
		"postgresql": aboveThresholdCount,
	}, result)
}

func TestScraper_ProcessActiveProfessionsDaily_Success(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go", "python"}, Description: "We need go developer"},
		{Skills: []string{"go", "python"}, Description: "We need go developer again"},
	}
	formalSkills := map[string]int{"go": 2, "python": 2}
	extractedSkills := map[string]int{"go": 2}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 100, nil)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).
		Return(map[string]int{"go": 1}, nil).
		Twice()
	deps.resultSaver.EXPECT().SaveDailyProfessionResult(
		mock.Anything,
		matchScrapedAt(),
		matchScrapeResult(professionID, 100, formalSkills, extractedSkills),
	).Return(nil)
	deps.cache.EXPECT().SaveProfessionData(mock.Anything, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID &&
			data.ProfessionName == "Go Developer" &&
			data.VacancyCount == 100 &&
			len(data.FormalSkills) == 2 &&
			len(data.ExtractedSkills) == 1
	})).Return(nil)

	err := deps.scraper().ProcessActiveProfessionsDaily(ctx)

	require.NoError(t, err)
	deps.sessionProvider.AssertNotCalled(t, "CreateScrapingSession")
}

func TestScraper_ProcessActiveProfessionsArchive_Success(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	sessionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer needed"},
		{Skills: []string{"go"}, Description: "Go developer needed"},
	}
	formalSkills := map[string]int{"go": 2}
	extractedSkills := map[string]int{"go": 2}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).
		Return(map[string]int{"go": 1}, nil).
		Twice()
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.resultSaver.EXPECT().SaveArchiveProfessionResult(
		mock.Anything,
		sessionID,
		matchScrapedAt(),
		matchScrapeResult(professionID, 50, formalSkills, extractedSkills),
	).Return(nil)
	deps.cache.EXPECT().SaveProfessionData(mock.Anything, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID && data.VacancyCount == 50
	})).Return(nil)

	err := deps.scraper().ProcessActiveProfessionsArchive(ctx)

	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsDaily_GetProfessionsError(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	dbError := assert.AnError
	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(nil, dbError).Times(retryMaxAttempts)

	err := deps.scraper().ProcessActiveProfessionsDaily(ctx)

	require.Error(t, err)
	deps.sessionProvider.AssertNotCalled(t, "CreateScrapingSession")
	deps.supplierPort.AssertNotCalled(t, "FetchDataProfession")
	deps.corpusProvider.AssertNotCalled(t, "GetActiveSkillCorpus")
	deps.resultSaver.AssertNotCalled(t, "SaveDailyProfessionResult")
	deps.extractor.AssertNotCalled(t, "ExtractSkills")
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsDaily_FetchDataError(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(nil, 0, assert.AnError)

	err := deps.scraper().ProcessActiveProfessionsDaily(ctx)

	require.NoError(t, err)
	deps.corpusProvider.AssertNotCalled(t, "GetActiveSkillCorpus")
	deps.resultSaver.AssertNotCalled(t, "SaveDailyProfessionResult")
	deps.extractor.AssertNotCalled(t, "ExtractSkills")
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsDaily_CorpusErrorFallsBackToFreshSkills(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(nil, assert.AnError).Times(retryMaxAttempts)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.MatchedBy(func(whitelist map[string]int) bool {
		return whitelist["go"] == 3
	}), 3).Return(map[string]int{"go": 1}, nil).Times(3)
	deps.resultSaver.EXPECT().SaveDailyProfessionResult(
		mock.Anything,
		matchScrapedAt(),
		matchScrapeResult(professionID, 50, map[string]int{"go": 3}, map[string]int{"go": 3}),
	).Return(nil)
	deps.cache.EXPECT().SaveProfessionData(mock.Anything, mock.Anything).Return(nil)

	err := deps.scraper().ProcessActiveProfessionsDaily(ctx)

	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsArchive_SaveResultErrorSkipsCache(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	sessionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 1}, nil).Twice()
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.resultSaver.EXPECT().SaveArchiveProfessionResult(
		mock.Anything,
		sessionID,
		matchScrapedAt(),
		matchScrapeResult(professionID, 50, map[string]int{"go": 2}, map[string]int{"go": 2}),
	).Return(assert.AnError).Times(retryMaxAttempts)

	err := deps.scraper().ProcessActiveProfessionsArchive(ctx)

	require.NoError(t, err)
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsArchive_CreateSessionError(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 1}, nil).Twice()
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(uuid.Nil, assert.AnError).Times(retryMaxAttempts)

	err := deps.scraper().ProcessActiveProfessionsArchive(ctx)

	require.Error(t, err)
	deps.resultSaver.AssertNotCalled(t, "SaveArchiveProfessionResult")
	deps.cache.AssertNotCalled(t, "SaveProfessionData")
}

func TestScraper_ProcessActiveProfessionsDaily_CacheErrorDoesNotRetryDBSave(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 1}, nil).Twice()
	deps.resultSaver.EXPECT().SaveDailyProfessionResult(
		mock.Anything,
		matchScrapedAt(),
		matchScrapeResult(professionID, 50, map[string]int{"go": 2}, map[string]int{"go": 2}),
	).Return(nil).Once()
	deps.cache.EXPECT().SaveProfessionData(mock.Anything, mock.Anything).Return(assert.AnError).Times(retryMaxAttempts)

	err := deps.scraper().ProcessActiveProfessionsDaily(ctx)

	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsArchive_MixedSuccess(t *testing.T) {
	ctx := context.Background()
	deps := newDeps(t)

	professionID1 := uuid.New()
	professionID2 := uuid.New()
	sessionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID1, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
		{ID: professionID2, Name: "Python Developer", VacancyQuery: "python developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	deps.professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	deps.supplierPort.EXPECT().FetchDataProfession(ctx, "python developer", "113").Return(nil, 0, assert.AnError)
	deps.corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	deps.extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 1}, nil).Twice()
	deps.sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	deps.resultSaver.EXPECT().SaveArchiveProfessionResult(
		mock.Anything,
		sessionID,
		matchScrapedAt(),
		matchScrapeResult(professionID1, 50, map[string]int{"go": 2}, map[string]int{"go": 2}),
	).Return(nil)
	deps.cache.EXPECT().SaveProfessionData(mock.Anything, mock.MatchedBy(func(data *domain.ProfessionDetail) bool {
		return data.ProfessionID == professionID1 && data.VacancyCount == 50
	})).Return(nil)

	err := deps.scraper().ProcessActiveProfessionsArchive(ctx)

	require.NoError(t, err)
}

func TestScraper_ProcessActiveProfessionsArchive_NilCache(t *testing.T) {
	ctx := context.Background()

	professionProvider := mocks.NewMockProfessionProvider(t)
	sessionProvider := mocks.NewMockSessionProvider(t)
	corpusProvider := mocks.NewMockSkillCorpusProvider(t)
	resultSaver := mocks.NewMockScrapeResultSaver(t)
	supplierPort := mocks.NewMockSupplierPort(t)
	extractor := mocks.NewMockExtractor(t)

	professionID := uuid.New()
	sessionID := uuid.New()
	professions := []domain.Profession{
		{ID: professionID, Name: "Go Developer", VacancyQuery: "go developer", IsActive: true},
	}
	vacancyData := []domain.VacancyData{
		{Skills: []string{"go"}, Description: "Go developer"},
		{Skills: []string{"go"}, Description: "Go developer"},
	}

	professionProvider.EXPECT().GetActiveProfessions(ctx).Return(professions, nil)
	supplierPort.EXPECT().FetchDataProfession(ctx, "go developer", "113").Return(vacancyData, 50, nil)
	corpusProvider.EXPECT().GetActiveSkillCorpus(ctx).Return(map[uuid.UUID]map[string]int{}, nil)
	extractor.EXPECT().ExtractSkills(mock.Anything, mock.Anything, 3).Return(map[string]int{"go": 1}, nil).Twice()
	sessionProvider.EXPECT().CreateScrapingSession(ctx).Return(sessionID, nil)
	resultSaver.EXPECT().SaveArchiveProfessionResult(
		mock.Anything,
		sessionID,
		matchScrapedAt(),
		matchScrapeResult(professionID, 50, map[string]int{"go": 2}, map[string]int{"go": 2}),
	).Return(nil)

	scraperService := New(
		professionProvider,
		sessionProvider,
		corpusProvider,
		resultSaver,
		supplierPort,
		extractor,
		nil,
	)

	err := scraperService.ProcessActiveProfessionsArchive(ctx)

	require.NoError(t, err)
}
