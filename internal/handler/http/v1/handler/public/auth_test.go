package public_test

import (
	"bytes"
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
	"psa/internal/handler/http/v1/handler/public"
	"psa/internal/handler/http/v1/handler/public/mocks"
)

// testDeps содержит зависимости для тестирования AuthHandler
type testDeps struct {
	auth *mocks.MockAuthenticator
}

func newDeps(t *testing.T) testDeps {
	t.Helper()
	return testDeps{
		auth: mocks.NewMockAuthenticator(t),
	}
}

func (d testDeps) handler() *public.AuthHandler {
	return public.NewAuthHandler(d.auth)
}

func doRequest(t *testing.T, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/signin", bytes.NewReader(reqBody))
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

// ==================== Signin ====================

func TestAuthHandler_Signin_Unit_Success(t *testing.T) {
	t.Parallel()

	email := "test@example.com"
	password := "password123"

	// Arrange
	deps := newDeps(t)

	tokenPair := &domain.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
	}

	deps.auth.EXPECT().Signin(mock.Anything, email, password).Return(tokenPair, nil)

	h := handler.Handle(deps.handler().Signin)

	// Act
	rr := doRequest(t, h, map[string]string{
		"email":    email,
		"password": password,
	})

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "access-token", resp["access_token"])
	assert.Equal(t, "refresh-token", resp["refresh_token"])
}

func TestAuthHandler_Signin_Unit_InvalidCredentials(t *testing.T) {
	t.Parallel()

	email := "test@example.com"
	password := "wrong-password"

	// Arrange
	deps := newDeps(t)

	deps.auth.EXPECT().Signin(mock.Anything, email, password).Return(nil, errors.New("invalid credentials"))

	h := handler.Handle(deps.handler().Signin)

	// Act
	rr := doRequest(t, h, map[string]string{
		"email":    email,
		"password": password,
	})

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Invalid credentials", resp["error"])
}

func TestAuthHandler_Signin_Unit_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	h := handler.Handle(deps.handler().Signin)

	// Act — невалидный JSON
	req := httptest.NewRequest(http.MethodPost, "/auth/signin", bytes.NewReader([]byte(`{invalid}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Contains(t, resp["error"], "invalid JSON")
}

// ==================== Refresh ====================

func TestAuthHandler_Refresh_Unit_Success(t *testing.T) {
	t.Parallel()

	refreshToken := "valid-refresh-token"

	// Arrange
	deps := newDeps(t)

	tokenPair := &domain.TokenPair{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
	}

	deps.auth.EXPECT().RefreshTokens(mock.Anything, refreshToken).Return(tokenPair, nil)

	h := handler.Handle(deps.handler().Refresh)

	// Act
	rr := doRequest(t, h, map[string]string{
		"refresh_token": refreshToken,
	})

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "new-access-token", resp["access_token"])
	assert.Equal(t, "new-refresh-token", resp["refresh_token"])
}

func TestAuthHandler_Refresh_Unit_InvalidToken(t *testing.T) {
	t.Parallel()

	refreshToken := "expired-token"

	// Arrange
	deps := newDeps(t)

	deps.auth.EXPECT().RefreshTokens(mock.Anything, refreshToken).Return(nil, errors.New("invalid token"))

	h := handler.Handle(deps.handler().Refresh)

	// Act
	rr := doRequest(t, h, map[string]string{
		"refresh_token": refreshToken,
	})

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Invalid refresh token", resp["error"])
}

// ==================== Logout ====================

func TestAuthHandler_Logout_Unit_Success(t *testing.T) {
	t.Parallel()

	refreshToken := "valid-refresh-token"

	// Arrange
	deps := newDeps(t)

	deps.auth.EXPECT().Logout(mock.Anything, refreshToken).Return(nil)

	h := handler.Handle(deps.handler().Logout)

	// Act
	rr := doRequest(t, h, map[string]string{
		"refresh_token": refreshToken,
	})

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Successfully logged out", resp["message"])
}

func TestAuthHandler_Logout_Unit_ServiceError(t *testing.T) {
	t.Parallel()

	refreshToken := "valid-refresh-token"

	// Arrange
	deps := newDeps(t)

	dbErr := errors.New("database connection failed")
	deps.auth.EXPECT().Logout(mock.Anything, refreshToken).Return(dbErr)

	h := handler.Handle(deps.handler().Logout)

	// Act
	rr := doRequest(t, h, map[string]string{
		"refresh_token": refreshToken,
	})

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "Failed to logout", resp["error"])
}

func TestAuthHandler_Logout_Unit_EmptyBody(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	h := handler.Handle(deps.handler().Logout)

	// Act
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, "request body is required", resp["error"])
}

// ==================== Edge cases ====================

func TestAuthHandler_Signin_Unit_MissingRequiredField(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	h := handler.Handle(deps.handler().Signin)

	// Act — тело без email
	body := map[string]string{"password": "password123"}
	rr := doRequest(t, h, body)

	// Assert — валидация должна вернуть 400, сервис не вызывается
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Signin_Unit_NilHandler(t *testing.T) {
	t.Parallel()

	// Arrange — пустой хендлер (authenticator nil)
	hh := &public.AuthHandler{}
	h := handler.Handle(hh.Signin)

	// Act
	req := httptest.NewRequest(http.MethodPost, "/auth/signin", bytes.NewReader([]byte(`{"email":"a","password":"b"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Assert — паник при nil authenticator
	assert.Panics(t, func() {
		h.ServeHTTP(rr, req)
	})
}

// ==================== PathUUID ====================

func TestAuthHandler_PathUUID_Unit_InvalidUUID(t *testing.T) {
	t.Parallel()

	// Arrange — хендлер который использует PathUUID
	h := handler.Handle(func(w http.ResponseWriter, r *http.Request) error {
		_, err := handler.PathUUID(r, "id")
		return err
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/latest", h.ServeHTTP)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/professions/not-a-uuid/latest", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Contains(t, resp["error"], "invalid uuid")
}

func TestAuthHandler_PathUUID_Unit_ValidUUID(t *testing.T) {
	t.Parallel()

	// Arrange
	h := handler.Handle(func(w http.ResponseWriter, r *http.Request) error {
		id, err := handler.PathUUID(r, "id")
		if err != nil {
			return err
		}
		handler.RespondJSON(w, http.StatusOK, map[string]string{"id": id.String()})
		return nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /professions/{id}/latest", h.ServeHTTP)

	// Act
	professionID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/professions/"+professionID+"/latest", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	decodeResponse(t, rr, &resp)
	assert.Equal(t, professionID, resp["id"])
}
