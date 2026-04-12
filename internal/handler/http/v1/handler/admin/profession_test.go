package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/internal/handler/http/v1/handler/admin"
	"psa/internal/handler/http/v1/handler/admin/mocks"
	cronMocks "psa/internal/service/cron/mocks"
)

// testDeps содержит зависимости для тестирования ProfessionAdminHandler
type testDeps struct {
	profession *mocks.MockProfessionAdminAccesser
	scraping   *cronMocks.MockScrapingProvider
}

func newDeps(t *testing.T) testDeps {
	t.Helper()
	return testDeps{
		profession: mocks.NewMockProfessionAdminAccesser(t),
		scraping:   cronMocks.NewMockScrapingProvider(t),
	}
}

func (d testDeps) handler() *admin.ProfessionAdminHandler {
	return admin.NewProfessionAdminHandler(d.profession, d.scraping)
}

func doRequest(t *testing.T, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/professions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	return rr
}

func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	err := json.Unmarshal(rr.Body.Bytes(), v)
	require.NoError(t, err)
}

// ==================== Create ====================

func TestProfessionAdminHandler_Create_Unit_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	professionID := uuid.New()

	deps.profession.EXPECT().CreateProfession(mock.Anything, domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "go developer OR golang",
		IsActive:     true,
	}).Return(professionID, nil)

	h := handler.Handle(deps.handler().Create)

	// Act
	rr := doRequest(t, h, map[string]string{
		"name":          "Go Developer",
		"vacancy_query": "go developer OR golang",
	})

	// Assert
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Go Developer", resp["name"])
	assert.Equal(t, "go developer OR golang", resp["vacancy_query"])
	assert.Equal(t, true, resp["is_active"])
}

func TestProfessionAdminHandler_Create_Unit_EmptyName(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	h := handler.Handle(deps.handler().Create)

	// Act
	rr := doRequest(t, h, map[string]string{
		"name":          "",
		"vacancy_query": "go developer",
	})

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Name is required", resp["error"])
}

func TestProfessionAdminHandler_Create_Unit_WhitespaceOnlyName(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	h := handler.Handle(deps.handler().Create)

	// Act
	rr := doRequest(t, h, map[string]string{
		"name":          "   ",
		"vacancy_query": "go developer",
	})

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Name is required", resp["error"])
}

func TestProfessionAdminHandler_Create_Unit_EmptyVacancyQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	h := handler.Handle(deps.handler().Create)

	// Act
	rr := doRequest(t, h, map[string]string{
		"name":          "Go Developer",
		"vacancy_query": "",
	})

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Vacancy query is required", resp["error"])
}

func TestProfessionAdminHandler_Create_Unit_Conflict(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	deps.profession.EXPECT().CreateProfession(mock.Anything, domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "golang",
		IsActive:     true,
	}).Return(uuid.Nil, domain.ErrProfessionAlreadyExists)

	h := handler.Handle(deps.handler().Create)

	// Act
	rr := doRequest(t, h, map[string]string{
		"name":          "Go Developer",
		"vacancy_query": "golang",
	})

	// Assert
	assert.Equal(t, http.StatusConflict, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Profession already exists", resp["error"])
}

func TestProfessionAdminHandler_Create_Unit_InternalError(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	dbErr := errors.New("database connection failed")
	deps.profession.EXPECT().CreateProfession(mock.Anything, domain.Profession{
		Name:         "Go Developer",
		VacancyQuery: "golang",
		IsActive:     true,
	}).Return(uuid.Nil, dbErr)

	h := handler.Handle(deps.handler().Create)

	// Act
	rr := doRequest(t, h, map[string]string{
		"name":          "Go Developer",
		"vacancy_query": "golang",
	})

	// Assert — должен быть 500, не 409
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Failed to create profession", resp["error"])
}

// ==================== TriggerScraping ====================

func TestProfessionAdminHandler_TriggerArchiveScraping_Unit_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	done := make(chan struct{})
	deps.scraping.EXPECT().
		ProcessActiveProfessionsArchive(mock.AnythingOfType("*context.timerCtx")).
		RunAndReturn(func(ctx context.Context) error {
			close(done)
			return nil
		})

	h := handler.Handle(deps.handler().TriggerArchiveScraping)

	// Act
	req := httptest.NewRequest(http.MethodPost, "/admin/scraping/archive", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "started", resp["status"])
	assert.Equal(t, "archive", resp["mode"])

	// Ждём завершения goroutine
	<-done
}

func TestProfessionAdminHandler_TriggerCacheScraping_Unit_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	done := make(chan struct{})
	deps.scraping.EXPECT().
		ProcessActiveProfessionsDaily(mock.AnythingOfType("*context.timerCtx")).
		RunAndReturn(func(ctx context.Context) error {
			close(done)
			return nil
		})

	h := handler.Handle(deps.handler().TriggerCacheScraping)

	// Act
	req := httptest.NewRequest(http.MethodPost, "/admin/scraping/cache", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "started", resp["status"])
	assert.Equal(t, "cache", resp["mode"])

	// Ждём завершения goroutine
	<-done
}

func TestProfessionAdminHandler_TriggerScraping_Unit_ConcurrentBlocked(t *testing.T) {
	t.Parallel()

	// Arrange — создаём хендлер с моками (CAS сработает до вызова scraping)
	deps := newDeps(t)

	done := make(chan struct{})
	deps.scraping.EXPECT().
		ProcessActiveProfessionsArchive(mock.AnythingOfType("*context.timerCtx")).
		RunAndReturn(func(ctx context.Context) error {
			close(done)
			return nil
		})

	h := deps.handler()

	// Первый запуск — должен пройти
	req1 := httptest.NewRequest(http.MethodPost, "/admin/scraping/archive", nil)
	rr1 := httptest.NewRecorder()

	handler.Handle(h.TriggerArchiveScraping).ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusAccepted, rr1.Code)

	// Второй запуск — должен быть заблокирован (CAS)
	req2 := httptest.NewRequest(http.MethodPost, "/admin/scraping/cache", nil)
	rr2 := httptest.NewRecorder()

	handler.Handle(h.TriggerCacheScraping).ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusConflict, rr2.Code)

	var resp map[string]string
	decodeResponse(t, rr2, &resp)
	assert.Equal(t, "Scraping already in progress", resp["error"])

	// Ждём завершения goroutine
	<-done
}

// ==================== Change ====================

func TestProfessionAdminHandler_Change_Unit_Success(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	adminDeps := newDeps(t)

	adminDeps.profession.EXPECT().ChangeProfession(mock.Anything, domain.Profession{
		ID:           professionUUID,
		Name:         "Updated Go Developer",
		VacancyQuery: "go developer OR golang OR go lang",
		IsActive:     false,
	}).Return(nil)

	h := handler.Handle(adminDeps.handler().Change)

	// Act
	req := httptest.NewRequest(http.MethodPut, "/admin/professions/"+professionUUID.String(), bytes.NewReader([]byte(`{
		"name": "Updated Go Developer",
		"vacancy_query": "go developer OR golang OR go lang",
		"is_active": false
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /admin/professions/{id}", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	decodeResponse(t, rr, &resp)
	assert.Equal(t, professionUUID.String(), resp["id"])
	assert.Equal(t, "Updated Go Developer", resp["name"])
	assert.Equal(t, "go developer OR golang OR go lang", resp["vacancy_query"])
	assert.Equal(t, false, resp["is_active"])
}

func TestProfessionAdminHandler_Change_Unit_InvalidUUID(t *testing.T) {
	t.Parallel()

	// Arrange
	adminDeps := newDeps(t)

	h := handler.Handle(adminDeps.handler().Change)

	// Act
	req := httptest.NewRequest(http.MethodPut, "/admin/professions/not-a-uuid", bytes.NewReader([]byte(`{
		"name": "Test",
		"vacancy_query": "test",
		"is_active": true
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /admin/professions/{id}", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Contains(t, resp["error"], "invalid uuid")
}

func TestProfessionAdminHandler_Change_Unit_InvalidJSON(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	adminDeps := newDeps(t)

	h := handler.Handle(adminDeps.handler().Change)

	// Act
	req := httptest.NewRequest(http.MethodPut, "/admin/professions/"+professionUUID.String(), bytes.NewReader([]byte(`{invalid}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /admin/professions/{id}", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Contains(t, resp["error"], "invalid JSON")
}

func TestProfessionAdminHandler_Change_Unit_InternalError(t *testing.T) {
	t.Parallel()

	professionUUID := uuid.New()

	// Arrange
	adminDeps := newDeps(t)

	dbErr := errors.New("database connection failed")
	adminDeps.profession.EXPECT().ChangeProfession(mock.Anything, domain.Profession{
		ID:           professionUUID,
		Name:         "Go Developer",
		VacancyQuery: "golang",
		IsActive:     true,
	}).Return(dbErr)

	h := handler.Handle(adminDeps.handler().Change)

	// Act
	req := httptest.NewRequest(http.MethodPut, "/admin/professions/"+professionUUID.String(), bytes.NewReader([]byte(`{
		"name": "Go Developer",
		"vacancy_query": "golang",
		"is_active": true
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /admin/professions/{id}", h.ServeHTTP)
	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Failed to change profession", resp["error"])
}

// ==================== ListAllProfessions ====================

func TestProfessionAdminHandler_ListAllProfessions_Unit_Success(t *testing.T) {
	t.Parallel()

	professionUUID1 := uuid.New()
	professionUUID2 := uuid.New()

	// Arrange
	adminDeps := newDeps(t)

	allProfs := []domain.Profession{
		{
			ID:           professionUUID1,
			Name:         "Go Developer",
			VacancyQuery: "golang OR go developer",
			IsActive:     true,
		},
		{
			ID:           professionUUID2,
			Name:         "Python Developer",
			VacancyQuery: "python OR django",
			IsActive:     false,
		},
	}

	adminDeps.profession.EXPECT().AllProfessions(mock.Anything).Return(allProfs, nil)

	h := handler.Handle(adminDeps.handler().ListAllProfessions)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/admin/professions", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	decodeResponse(t, rr, &resp)
	assert.Len(t, resp, 2)
	assert.Equal(t, professionUUID1.String(), resp[0]["id"])
	assert.Equal(t, "Go Developer", resp[0]["name"])
	assert.Equal(t, "golang OR go developer", resp[0]["vacancy_query"])
	assert.Equal(t, true, resp[0]["is_active"])
	assert.Equal(t, professionUUID2.String(), resp[1]["id"])
	assert.Equal(t, "Python Developer", resp[1]["name"])
	assert.Equal(t, false, resp[1]["is_active"])
}

func TestProfessionAdminHandler_ListAllProfessions_Unit_EmptyList(t *testing.T) {
	t.Parallel()

	// Arrange
	adminDeps := newDeps(t)

	adminDeps.profession.EXPECT().AllProfessions(mock.Anything).Return([]domain.Profession{}, nil)

	h := handler.Handle(adminDeps.handler().ListAllProfessions)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/admin/professions", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	decodeResponse(t, rr, &resp)
	assert.Empty(t, resp)
}

func TestProfessionAdminHandler_ListAllProfessions_Unit_InternalError(t *testing.T) {
	t.Parallel()

	// Arrange
	adminDeps := newDeps(t)

	dbErr := errors.New("database connection failed")
	adminDeps.profession.EXPECT().AllProfessions(mock.Anything).Return(nil, dbErr)

	h := handler.Handle(adminDeps.handler().ListAllProfessions)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/admin/professions", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Failed to get all professions", resp["error"])
}
