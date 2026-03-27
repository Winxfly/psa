package hh

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"psa/internal/config"
)

// newTestLogger создаёт тестовый логгер
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// testWriter — заглушка для логов в тестах
type testWriter struct{}

func (w *testWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// newTestTokenManager создаёт tokenManager с кастомным baseURL для тестов
func newTestTokenManager(serverURL string, logger *slog.Logger, pregeneratedToken string) *tokenManager {
	cfg := config.HHAuth{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		AccessToken:  pregeneratedToken,
		UserAgent:    "test-agent",
	}

	tm := newTokenManager(cfg, logger)
	tm.baseURL = serverURL

	return tm
}

func TestTokenManager_GetToken_Cached(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём токен-менеджер с предгенерированным токеном
	tm := newTestTokenManager("", logger, "cached-token-123")
	// Сбрасываем lastRefresh чтобы не было ожидания
	tm.lastRefresh = time.Now()

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "cached-token-123", token)
}

func TestTokenManager_GetToken_Refresh(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём тестовый сервер который эмулирует hh.ru OAuth API
	var reqBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Сохраняем данные запроса для проверки в основном тесте
		reqBody = r.FormValue("grant_type") + ":" + r.FormValue("client_id") + ":" + r.FormValue("client_secret")

		// Возвращаем успешный ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "new-test-token-456"}`))
	}))
	defer server.Close()

	// Создаём токен-менеджер без токена (нужен refresh)
	tm := newTestTokenManager(server.URL, logger, "")
	// Устанавливаем lastRefresh в прошлое чтобы не было ожидания
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "new-test-token-456", token)
	assert.False(t, tm.refreshFailed)
	// Проверяем что запрос был с правильными параметрами
	assert.Equal(t, "client_credentials:test-client-id:test-client-secret", reqBody)
}

func TestTokenManager_GetToken_EmptyConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём тестовый сервер который возвращает ошибку при пустых креденшалах
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		// Если client_id или client_secret пустые — возвращаем ошибку
		if r.FormValue("client_id") == "" || r.FormValue("client_secret") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "test-token"}`))
	}))
	defer server.Close()

	// Создаём токен-менеджер с пустыми креденшалами
	cfg := config.HHAuth{
		ClientID:     "",
		ClientSecret: "",
	}
	tm := newTokenManager(cfg, logger)
	tm.baseURL = server.URL

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.Error(t, err)
	assert.Empty(t, token)
	assert.True(t, tm.refreshFailed)
}

func TestTokenManager_GetToken_HTTPError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём тестовый сервер который возвращает 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.Error(t, err)
	assert.Empty(t, token)
	assert.True(t, tm.refreshFailed)
}

func TestTokenManager_GetToken_InvalidResponse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём тестовый сервер который возвращает невалидный JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"invalid": "json"}`)) // нет access_token
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.Error(t, err)
	assert.Empty(t, token)
	assert.True(t, tm.refreshFailed)
}

func TestTokenManager_GetToken_EmptyAccessToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём тестовый сервер который возвращает пустой access_token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": ""}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty access token")
	assert.Empty(t, token)
	assert.True(t, tm.refreshFailed)
}

func TestTokenManager_handleAuthError(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()

	tm := newTestTokenManager("", logger, "some-token")
	tm.refreshFailed = false

	// Act
	tm.handleAuthError()

	// Assert
	assert.True(t, tm.refreshFailed)
}

func TestTokenManager_GetToken_CacheAfterRefresh(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	requestCount := 0

	// Создаём тестовый сервер который считает запросы
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "test-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	// Устанавливаем lastRefresh в прошлое чтобы не было ожидания cooldown
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Act — первый вызов
	token1, err1 := tm.getToken(ctx)

	// Assert
	require.NoError(t, err1)
	assert.Equal(t, "test-token", token1)
	assert.Equal(t, 1, requestCount)

	// Второй вызов сразу — должен вернуть кэшированный токен без запроса
	token2, err2 := tm.getToken(ctx)

	require.NoError(t, err2)
	assert.Equal(t, "test-token", token2)
	assert.Equal(t, 1, requestCount) // запрос не увеличился, токен из кэша
}

func TestTokenManager_GetToken_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	logger := newTestLogger()

	// Создаём тестовый сервер который ждёт и проверяет контекст запроса
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ждём либо отмены контекста запроса, либо таймаута
		select {
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			// Контекст отменён — завершаем без ответа
			return
		}
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Отменяем контекст сразу
	cancel()

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.Error(t, err)
	assert.Empty(t, token)
}

func TestTokenManager_GetToken_RefreshSuccess_ResetsFlag(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "new-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)
	tm.refreshFailed = true // Предыдущий refresh не удался

	// Act
	token, err := tm.getToken(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
	assert.False(t, tm.refreshFailed) // Флаг сброшен после успеха
}

func TestTokenManager_HandleAuthError_ThenGetToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	requestCount := 0

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "refreshed-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "old-token")
	tm.lastRefresh = time.Now().Add(-time.Hour)
	tm.refreshFailed = false

	// Имитируем ошибку авторизации
	tm.handleAuthError()

	// Assert — флаг установлен
	assert.True(t, tm.refreshFailed)

	// Act — следующий getToken должен сделать refresh
	token, err := tm.getToken(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "refreshed-token", token)
	assert.Equal(t, 1, requestCount)  // был один запрос на refresh
	assert.False(t, tm.refreshFailed) // флаг сброшен после успеха
}

func TestTokenManager_GetToken_Parallel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	requestCount := 0
	var mu sync.Mutex

	// Создаём тестовый сервер который считает запросы
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "parallel-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)
	tm.refreshFailed = true // Форсируем refresh

	// Запускаем 100 горуттин одновременно
	const goroutines = 100
	var wg sync.WaitGroup
	tokens := make([]string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, err := tm.getToken(ctx)
			require.NoError(t, err)
			tokens[idx] = token
		}(i)
	}

	wg.Wait()

	// Assert — все получили один токен, HTTP вызов = 1
	assert.Equal(t, 1, requestCount)

	for i := 1; i < goroutines; i++ {
		assert.Equal(t, tokens[0], tokens[i])
	}
}
