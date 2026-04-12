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

// trendTestDeps содержит зависимости для тестирования TrendHandler
type trendTestDeps struct {
	trendProvider *mocks.MockTrendProvider
}

func newTrendDeps(t *testing.T) trendTestDeps {
	t.Helper()
	return trendTestDeps{
		trendProvider: mocks.NewMockTrendProvider(t),
	}
}

func (d trendTestDeps) trendHandler() *public.TrendHandler {
	return public.NewTrendHandler(d.trendProvider)
}

func decodeTrendResponse(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	err := json.Unmarshal(rr.Body.Bytes(), v)
	require.NoError(t, err)
}

func routeTrend(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/trend", h.ServeHTTP)
	return mux
}

// ==================== GetProfessionTrend ====================

func TestTrendHandler_GetProfessionTrend_Unit_Success(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()
	trendDate1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	trendDate2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	// Arrange
	trendDeps := newTrendDeps(t)

	trendData := &domain.ProfessionTrend{
		ProfessionID:   professionUUID,
		ProfessionName: "Go Developer",
		Data: []domain.StatDailyPoint{
			{Date: trendDate1, VacancyCount: 100},
			{Date: trendDate2, VacancyCount: 120},
		},
	}

	trendDeps.trendProvider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(trendData, nil)

	h := handler.Handle(trendDeps.trendHandler().GetProfessionTrend)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/trend", nil)
	rr := httptest.NewRecorder()

	routeTrend(h).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeTrendResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["profession_id"])
	assert.Equal(t, "Go Developer", resp["profession_name"])

	data := resp["data"].([]any)
	assert.Len(t, data, 2)
	assert.Equal(t, trendDate1.Format(time.RFC3339), data[0].(map[string]any)["date"])
	assert.Equal(t, float64(100), data[0].(map[string]any)["vacancy_count"])
	assert.Equal(t, trendDate2.Format(time.RFC3339), data[1].(map[string]any)["date"])
	assert.Equal(t, float64(120), data[1].(map[string]any)["vacancy_count"])
}

func TestTrendHandler_GetProfessionTrend_Unit_EmptyTrend(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	trendDeps := newTrendDeps(t)

	trendData := &domain.ProfessionTrend{
		ProfessionID:   professionUUID,
		ProfessionName: "Go Developer",
		Data:           []domain.StatDailyPoint{},
	}

	trendDeps.trendProvider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(trendData, nil)

	h := handler.Handle(trendDeps.trendHandler().GetProfessionTrend)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/trend", nil)
	rr := httptest.NewRecorder()

	routeTrend(h).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeTrendResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["profession_id"])

	data := resp["data"].([]any)
	assert.Len(t, data, 0)
}

func TestTrendHandler_GetProfessionTrend_Unit_InvalidUUID(t *testing.T) {
	t.Parallel()

	// Arrange
	trendDeps := newTrendDeps(t)

	h := handler.Handle(trendDeps.trendHandler().GetProfessionTrend)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/invalid-uuid/trend", nil)
	rr := httptest.NewRecorder()

	routeTrend(h).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeTrendResponse(t, rr, &resp)
	assert.Contains(t, resp["error"], "Invalid profession ID")
}

func TestTrendHandler_GetProfessionTrend_Unit_NotFound(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	trendDeps := newTrendDeps(t)

	trendDeps.trendProvider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(nil, domain.ErrProfessionNotFound)

	h := handler.Handle(trendDeps.trendHandler().GetProfessionTrend)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/trend", nil)
	rr := httptest.NewRecorder()

	routeTrend(h).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Code)

	var resp map[string]string
	decodeTrendResponse(t, rr, &resp)
	assert.Equal(t, "Profession trend not found", resp["error"])
}

func TestTrendHandler_GetProfessionTrend_Unit_InternalError(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	trendDeps := newTrendDeps(t)

	trendDeps.trendProvider.EXPECT().ProfessionTrend(mock.Anything, professionUUID).Return(nil, assert.AnError)

	h := handler.Handle(trendDeps.trendHandler().GetProfessionTrend)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionUUID.String()+"/trend", nil)
	rr := httptest.NewRecorder()

	routeTrend(h).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeTrendResponse(t, rr, &resp)
	assert.Equal(t, "Failed to get profession trend", resp["error"])
}
