package hh

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"psa/internal/config"
)

// newTestConfig создаёт тестовую конфигурацию
func newTestConfig() *config.Config {
	return &config.Config{
		HHAuth: config.HHAuth{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			UserAgent:    "test-agent",
		},
		HHRetry: config.HHRetry{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			MaxTotalTime: 5 * time.Second,
		},
	}
}

// newTestLogger создаёт тестовый логгер
func newTestClientLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&testClientWriter{}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// testWriter — заглушка для логов в тестах
type testClientWriter struct{}

func (w *testClientWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// newTestClient создаёт client с кастомным baseURL для тестов
func newTestClient(baseURL string, cfg *config.Config, logger *slog.Logger, token tokenProvider) *client {
	c := newClient(cfg, logger, &http.Client{}, token)
	c.baseURL = baseURL
	return c
}

// mockTokenProvider реализует tokenProvider для тестов
type mockTokenProvider struct {
	mu            sync.Mutex
	token         string
	err           error
	handleErrCall int
}

func (m *mockTokenProvider) getToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token, m.err
}

func (m *mockTokenProvider) handleAuthError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handleErrCall++
}

// TestClient_SetHeaders тестирует установку заголовков
func TestClient_SetHeaders(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token-123"}

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://test.com", nil)
	require.NoError(t, err)

	err = c.setHeaders(req)

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token-123", req.Header.Get("Authorization"))
	assert.Equal(t, "test-agent", req.Header.Get("User-Agent"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}

// TestClient_SetHeaders_TokenError тестирует ошибку получения токена
func TestClient_SetHeaders_TokenError(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{err: fmt.Errorf("token error")}

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://test.com", nil)
	require.NoError(t, err)

	err = c.setHeaders(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get token")
}

// TestClient_CalculateWait тестирует расчёт задержки
func TestClient_CalculateWait(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{}

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	tests := []struct {
		name     string
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{"attempt 0", 0, 0, 0},
		{"attempt 1", 1, 0, 20 * time.Millisecond},
		{"attempt 2", 2, 10 * time.Millisecond, 30 * time.Millisecond},
		{"attempt 3", 3, 20 * time.Millisecond, 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := c.calculateWait(tt.attempt)
			assert.GreaterOrEqual(t, delay, tt.minDelay)
			assert.LessOrEqual(t, delay, tt.maxDelay)
		})
	}
}

// TestClient_DoRequestWithRetry_Success тестирует успешный запрос
func TestClient_DoRequestWithRetry_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"test": "data"}`))
	}))
	defer server.Close()

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := c.doRequestWithRetry(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), requestCount)
	_ = resp.Body.Close()
}

// TestClient_DoRequestWithRetry_Retry500 тестирует retry при 500 ошибке
func TestClient_DoRequestWithRetry_Retry500(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := c.doRequestWithRetry(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.GreaterOrEqual(t, requestCount, int32(2))
	_ = resp.Body.Close()
}

// TestClient_DoRequestWithRetry_Retry403 тестирует retry при 403 с handleAuthError
func TestClient_DoRequestWithRetry_Retry403(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 2 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := c.doRequestWithRetry(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.GreaterOrEqual(t, requestCount, int32(2))
	assert.GreaterOrEqual(t, tokenProvider.handleErrCall, 1)
	_ = resp.Body.Close()
}

// TestClient_DoRequestWithRetry_MaxRetries тестирует превышение лимита retry
func TestClient_DoRequestWithRetry_MaxRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	//nolint:bodyclose // resp is nil on error
	resp, err := c.doRequestWithRetry(ctx, req)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.GreaterOrEqual(t, requestCount, int32(3))
}

// TestClient_DoRequestWithRetry_ContextCancelled тестирует отмену контекста
func TestClient_DoRequestWithRetry_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := newClient(cfg, logger, &http.Client{}, tokenProvider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Отменяем контекст сразу
	cancel()

	//nolint:bodyclose // resp is nil on error
	resp, err := c.doRequestWithRetry(ctx, req)
	require.Error(t, err)
	assert.Nil(t, resp)
}

// TestClient_IsRetryable тестирует функцию isRetryable
func TestClient_IsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		statusCode int
		expected   bool
	}{
		{http.StatusOK, false},
		{http.StatusTooManyRequests, true},
		{http.StatusForbidden, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusBadRequest, false},
		{http.StatusNotFound, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			assert.Equal(t, tt.expected, isRetryable(tt.statusCode))
		})
	}
}

// TestClient_FetchIDsVacancies_PartialFailure тестирует частичную ошибку при получении ID вакансий
func TestClient_FetchIDsVacancies_PartialFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")

		if page == "1" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		atomic.AddInt32(&calls, 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]string{
				{"id": "vacancy-1"},
				{"id": "vacancy-2"},
			},
		})
	}))
	defer server.Close()

	c := newTestClient(server.URL, cfg, logger, tokenProvider)

	meta := metadata{
		Found: 200,
		Pages: 2,
	}

	ids, err := c.fetchIDsVacancies(ctx, meta, "test", "1")

	require.NoError(t, err)
	assert.NotEmpty(t, ids)
	assert.Greater(t, len(ids), 0)
}

// TestClient_FetchIDsVacancies_AllFailed тестирует полный провал при получении ID вакансий
func TestClient_FetchIDsVacancies_AllFailed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := newTestClient(server.URL, cfg, logger, tokenProvider)

	meta := metadata{
		Found: 200,
		Pages: 2,
	}

	ids, err := c.fetchIDsVacancies(ctx, meta, "test", "1")

	require.Error(t, err)
	assert.Nil(t, ids)
	assert.Contains(t, err.Error(), "all pages failed")
}

// TestClient_FetchDataVacancies_PartialFailure тестирует частичную ошибку при получении данных вакансий
func TestClient_FetchDataVacancies_PartialFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path

		if id == "/vacancy-2" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		atomic.AddInt32(&calls, 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          id,
			"name":        "Test Vacancy",
			"description": "Test description",
			"key_skills":  []map[string]string{{"name": "Go"}},
		})
	}))
	defer server.Close()

	c := newTestClient(server.URL, cfg, logger, tokenProvider)

	ids := []string{"vacancy-1", "vacancy-2", "vacancy-3"}

	data, err := c.fetchDataVacancies(ctx, ids)

	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Greater(t, len(data), 0)
}

// TestClient_FetchDataVacancies_AllFailed тестирует полный провал при получении данных вакансий
func TestClient_FetchDataVacancies_AllFailed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := newTestConfig()
	logger := newTestClientLogger()
	tokenProvider := &mockTokenProvider{token: "test-token"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := newTestClient(server.URL, cfg, logger, tokenProvider)

	ids := []string{"vacancy-1", "vacancy-2"}

	data, err := c.fetchDataVacancies(ctx, ids)

	require.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "all vacancies fetch failed")
}
