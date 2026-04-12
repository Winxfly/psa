package public_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/internal/handler/http/v1/handler/public"
	"psa/internal/handler/http/v1/handler/public/mocks"
)

// profTestDeps содержит зависимости для тестирования ProfessionHandler
type profTestDeps struct {
	provider *mocks.MockProfessionProvider
}

func newProfDeps(t *testing.T) profTestDeps {
	t.Helper()
	return profTestDeps{
		provider: mocks.NewMockProfessionProvider(t),
	}
}

func (d profTestDeps) profHandler() *public.ProfessionHandler {
	return public.NewProfessionHandler(d.provider)
}

func doProfRequest(t *testing.T, h http.Handler, method, url string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, url, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	return rr
}

func decodeProfResponse(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	err := json.Unmarshal(rr.Body.Bytes(), v)
	require.NoError(t, err)
}

// ==================== ListProfessions ====================

func TestProfessionHandler_ListProfessions_Unit_Success(t *testing.T) {
	t.Parallel()

	professionUUID1 := uuid.New()
	professionUUID2 := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	activeProfs := []domain.ActiveProfession{
		{
			ID:           professionUUID1,
			Name:         "Go Developer",
			VacancyQuery: "golang OR go developer",
		},
		{
			ID:           professionUUID2,
			Name:         "Python Developer",
			VacancyQuery: "python OR django",
		},
	}

	profDeps.provider.EXPECT().ActiveProfessions(mock.Anything).Return(activeProfs, nil)

	h := handler.Handle(profDeps.profHandler().ListProfessions)

	// Act
	rr := doProfRequest(t, h, http.MethodGet, "/professions")

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	decodeProfResponse(t, rr, &resp)
	assert.Len(t, resp, 2)
	assert.Equal(t, professionUUID1.String(), resp[0]["id"])
	assert.Equal(t, "Go Developer", resp[0]["name"])
	assert.Equal(t, "golang OR go developer", resp[0]["vacancy_query"])
	assert.Equal(t, professionUUID2.String(), resp[1]["id"])
	assert.Equal(t, "Python Developer", resp[1]["name"])
	assert.Equal(t, "python OR django", resp[1]["vacancy_query"])
}

func TestProfessionHandler_ListProfessions_Unit_EmptyList(t *testing.T) {
	t.Parallel()

	// Arrange
	profDeps := newProfDeps(t)

	profDeps.provider.EXPECT().ActiveProfessions(mock.Anything).Return([]domain.ActiveProfession{}, nil)

	h := handler.Handle(profDeps.profHandler().ListProfessions)

	// Act
	rr := doProfRequest(t, h, http.MethodGet, "/professions")

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	decodeProfResponse(t, rr, &resp)
	assert.Empty(t, resp)
}

func TestProfessionHandler_ListProfessions_Unit_ServiceError(t *testing.T) {
	t.Parallel()

	// Arrange
	profDeps := newProfDeps(t)

	profDeps.provider.EXPECT().ActiveProfessions(mock.Anything).Return(nil, assert.AnError)

	h := handler.Handle(profDeps.profHandler().ListProfessions)

	// Act
	rr := doProfRequest(t, h, http.MethodGet, "/professions")

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, "Failed to get professions", resp["error"])
}

// ==================== LastProfessionDetails ====================

func TestProfessionHandler_LastProfessionDetails_Unit_Success(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	detail := &domain.ProfessionDetail{
		ProfessionID:   professionUUID,
		ProfessionName: "Go Developer",
		ScrapedAt:      "2024-01-01T00:00:00Z",
		VacancyCount:   150,
		FormalSkills: []domain.SkillResponse{
			{Skill: "Go", Count: 100},
			{Skill: "PostgreSQL", Count: 80},
		},
		ExtractedSkills: []domain.SkillResponse{
			{Skill: "Microservices", Count: 60},
		},
	}

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(detail, nil)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["profession_id"])
	assert.Equal(t, "Go Developer", resp["profession_name"])
	assert.Equal(t, "2024-01-01T00:00:00Z", resp["scraped_at"])
	assert.Equal(t, float64(150), resp["vacancy_count"])

	formalSkills := resp["formal_skills"].([]any)
	assert.Len(t, formalSkills, 2)
	assert.Equal(t, "Go", formalSkills[0].(map[string]any)["skill"])
	assert.Equal(t, float64(100), formalSkills[0].(map[string]any)["count"])

	extractedSkills := resp["extracted_skills"].([]any)
	assert.Len(t, extractedSkills, 1)
	assert.Equal(t, "Microservices", extractedSkills[0].(map[string]any)["skill"])
}

func TestProfessionHandler_LastProfessionDetails_Unit_SuccessWithTrend(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()
	trendDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Arrange
	profDeps := newProfDeps(t)

	detail := &domain.ProfessionDetail{
		ProfessionID:    professionUUID,
		ProfessionName:  "Go Developer",
		ScrapedAt:       "2024-01-01T00:00:00Z",
		VacancyCount:    150,
		FormalSkills:    []domain.SkillResponse{},
		ExtractedSkills: []domain.SkillResponse{},
	}

	trendData := &domain.ProfessionTrend{
		ProfessionID:   professionUUID,
		ProfessionName: "Go Developer",
		Data: []domain.StatDailyPoint{
			{Date: trendDate, VacancyCount: 100},
			{Date: trendDate.AddDate(0, 0, 1), VacancyCount: 120},
		},
	}

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(detail, nil)
	profDeps.provider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(trendData, nil)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details?trend=true", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["profession_id"])

	trend := resp["trend"].([]any)
	assert.Len(t, trend, 2)
	assert.Equal(t, trendDate.Format(time.RFC3339), trend[0].(map[string]any)["date"])
	assert.Equal(t, float64(100), trend[0].(map[string]any)["vacancy_count"])
	assert.Equal(t, trendDate.AddDate(0, 0, 1).Format(time.RFC3339), trend[1].(map[string]any)["date"])
	assert.Equal(t, float64(120), trend[1].(map[string]any)["vacancy_count"])
}

func TestProfessionHandler_LastProfessionDetails_Unit_WithoutTrend(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	detail := &domain.ProfessionDetail{
		ProfessionID:    professionUUID,
		ProfessionName:  "Go Developer",
		ScrapedAt:       "2024-01-01T00:00:00Z",
		VacancyCount:    150,
		FormalSkills:    []domain.SkillResponse{},
		ExtractedSkills: []domain.SkillResponse{},
	}

	// ProfessionTrend НЕ должен вызываться
	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(detail, nil)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act — запрос без query параметра trend
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["profession_id"])
	// trend не должен присутствовать в ответе
	_, hasTrend := resp["trend"]
	assert.False(t, hasTrend, "trend should not be present when trend=false")
}

func TestProfessionHandler_LastProfessionDetails_Unit_EmptyTrend(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	detail := &domain.ProfessionDetail{
		ProfessionID:    professionUUID,
		ProfessionName:  "Go Developer",
		ScrapedAt:       "2024-01-01T00:00:00Z",
		VacancyCount:    150,
		FormalSkills:    []domain.SkillResponse{},
		ExtractedSkills: []domain.SkillResponse{},
	}

	trendData := &domain.ProfessionTrend{
		ProfessionID:   professionUUID,
		ProfessionName: "Go Developer",
		Data:           []domain.StatDailyPoint{},
	}

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(detail, nil)
	profDeps.provider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(trendData, nil)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details?trend=true", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["profession_id"])

	// trend с omitempty — при пустых данных поле отсутствует в JSON
	_, hasTrend := resp["trend"]
	assert.False(t, hasTrend, "trend should be omitted when empty (omitempty behavior)")
}

func TestProfessionHandler_LastProfessionDetails_Unit_InvalidUUID(t *testing.T) {
	t.Parallel()

	// Arrange
	profDeps := newProfDeps(t)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/not-a-uuid/details", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeProfResponse(t, rr, &resp)
	assert.Contains(t, resp["error"], "Invalid profession ID")
}

func TestProfessionHandler_LastProfessionDetails_Unit_NotFound(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(nil, domain.ErrProfessionNotFound)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Code)

	var resp map[string]string
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, "Profession not found", resp["error"])
}

func TestProfessionHandler_LastProfessionDetails_Unit_ServiceError(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(nil, assert.AnError)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, "Failed to get profession details", resp["error"])
}

func TestProfessionHandler_LastProfessionDetails_Unit_TrendError_NotFound(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	detail := &domain.ProfessionDetail{
		ProfessionID:    professionUUID,
		ProfessionName:  "Go Developer",
		ScrapedAt:       "2024-01-01T00:00:00Z",
		VacancyCount:    150,
		FormalSkills:    []domain.SkillResponse{},
		ExtractedSkills: []domain.SkillResponse{},
	}

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(detail, nil)
	profDeps.provider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(nil, domain.ErrProfessionNotFound)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details?trend=true", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Code)

	var resp map[string]string
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, "Profession not found", resp["error"])
}

func TestProfessionHandler_LastProfessionDetails_Unit_TrendError_Internal(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	profDeps := newProfDeps(t)

	detail := &domain.ProfessionDetail{
		ProfessionID:    professionUUID,
		ProfessionName:  "Go Developer",
		ScrapedAt:       "2024-01-01T00:00:00Z",
		VacancyCount:    150,
		FormalSkills:    []domain.SkillResponse{},
		ExtractedSkills: []domain.SkillResponse{},
	}

	profDeps.provider.EXPECT().ProfessionSkills(mock.Anything, professionUUID).Return(detail, nil)
	profDeps.provider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(nil, assert.AnError)

	h := handler.Handle(profDeps.profHandler().LastProfessionDetails)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/details?trend=true", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/details", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeProfResponse(t, rr, &resp)
	assert.Equal(t, "Failed to get profession details", resp["error"])
}
