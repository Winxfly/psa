package hh

import (
	"context"
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

	tm := newTestTokenManager("", logger, "cached-token-123")
	tm.lastRefresh = time.Now()

	token, err := tm.getToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "cached-token-123", token)
}

func TestTokenManager_GetToken_Refresh(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var reqBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody = r.FormValue("grant_type") + ":" + r.FormValue("client_id") + ":" + r.FormValue("client_secret")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "new-test-token-456"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token, err := tm.getToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "new-test-token-456", token)
	assert.Equal(t, "client_credentials:test-client-id:test-client-secret", reqBody)
}

func TestTokenManager_GetToken_EmptyConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("client_id") == "" || r.FormValue("client_secret") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "test-token"}`))
	}))
	defer server.Close()

	cfg := config.HHAuth{
		ClientID:     "",
		ClientSecret: "",
	}
	tm := newTokenManager(cfg, logger)
	tm.baseURL = server.URL

	token, err := tm.getToken(ctx)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Empty(t, tm.accessToken)
}

func TestTokenManager_GetToken_HTTPError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token, err := tm.getToken(ctx)

	require.Error(t, err)
	assert.Empty(t, token)
	// retry 3 раза
	assert.Equal(t, int32(maxRefreshAttempts), callCount.Load())
}

func TestTokenManager_GetToken_InvalidResponse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"invalid": "json"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token, err := tm.getToken(ctx)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Empty(t, tm.accessToken)
}

func TestTokenManager_GetToken_EmptyAccessToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": ""}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token, err := tm.getToken(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
	assert.Empty(t, token)
	assert.Empty(t, tm.accessToken)
}

func TestTokenManager_handleAuthError(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()

	tm := newTestTokenManager("", logger, "some-token")

	tm.handleAuthError()

	assert.Empty(t, tm.accessToken)
}

func TestTokenManager_GetToken_CacheAfterRefresh(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "test-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token1, err1 := tm.getToken(ctx)
	require.NoError(t, err1)
	assert.Equal(t, "test-token", token1)
	assert.Equal(t, int32(1), requestCount.Load())

	// второй вызов — кэш
	token2, err2 := tm.getToken(ctx)
	require.NoError(t, err2)
	assert.Equal(t, "test-token", token2)
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestTokenManager_GetToken_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	cancel()

	token, err := tm.getToken(ctx)

	require.Error(t, err)
	assert.Empty(t, token)
}

func TestTokenManager_GetToken_RefreshSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "new-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token, err := tm.getToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
	assert.Equal(t, "new-token", tm.accessToken)
}

func TestTokenManager_HandleAuthError_ThenGetToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "refreshed-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "old-token")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	tm.handleAuthError()
	assert.Empty(t, tm.accessToken)

	token, err := tm.getToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "refreshed-token", token)
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestTokenManager_GetToken_Parallel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "parallel-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

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

	// singleflight гарантирует 1 HTTP-запрос
	assert.Equal(t, int32(1), requestCount.Load())

	for i := 1; i < goroutines; i++ {
		assert.Equal(t, tokens[0], tokens[i])
	}
}

// TestTokenManager_RecoversAfterTransientError — именно тот баг из прода:
// одна ошибка → перманентный отказ. Теперь: ошибка → cooldown → retry → успех.
func TestTokenManager_RecoversAfterTransientError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Первые 3 запроса (retry) падают, 4-й — успех
		if callCount.Load() <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "recovered-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Первый вызов — 3 retry → ошибка
	_, err := tm.getToken(ctx)
	require.Error(t, err)
	assert.Equal(t, int32(3), callCount.Load())

	// Второй вызов — после cooldown refresh сработает
	token, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "recovered-token", token)

	// Третий вызов — кэш
	token2, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "recovered-token", token2)
}

// TestTokenManager_RespectCooldown проверяет что refresh не вызывается
// слишком часто между успешными обновлениями.
func TestTokenManager_RespectCooldown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now()

	resultCh := make(chan string, 1)
	go func() {
		token, err := tm.getToken(ctx)
		if err == nil {
			resultCh <- token
		}
	}()

	select {
	case <-resultCh:
		t.Fatal("getToken вернул результат мгновенно, cooldown не сработал")
	case <-time.After(100 * time.Millisecond):
		// OK — всё ещё ждёт cooldown
	}

	<-ctx.Done()
}

// TestTokenManager_JSONEdgeCases проверяет edge cases декодинга JSON.
func TestTokenManager_JSONEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		response   string
		errContain string
	}{
		{
			name:     "null access_token",
			response: `{"access_token": null}`,
		},
		{
			name:       "empty object",
			response:   `{}`,
			errContain: "empty token",
		},
		{
			name:     "access_token as number",
			response: `{"access_token": 123}`,
		},
		{
			name:       "whitespace only",
			response:   `   `,
			errContain: "decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			logger := newTestLogger()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			tm := newTestTokenManager(server.URL, logger, "")
			tm.lastRefresh = time.Now().Add(-time.Hour)

			token, err := tm.getToken(ctx)

			require.Error(t, err)
			assert.Empty(t, token)
			assert.Empty(t, tm.accessToken)
			if tt.errContain != "" {
				assert.Contains(t, err.Error(), tt.errContain)
			}
		})
	}
}

func TestTokenManager_HandleAuthError_ThenCacheWorks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "fresh-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "old-token")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	tm.handleAuthError()
	assert.Empty(t, tm.accessToken)

	token, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "fresh-token", token)
	assert.Equal(t, int32(1), requestCount.Load())

	// кэш
	for i := 0; i < 5; i++ {
		token, err := tm.getToken(ctx)
		require.NoError(t, err)
		assert.Equal(t, "fresh-token", token)
	}
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestTokenManager_MultipleAuthErrorsAndRecovery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "recovered-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "initial-token")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	for i := 0; i < 3; i++ {
		tm.handleAuthError()
	}

	assert.Empty(t, tm.accessToken)

	token, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "recovered-token", token)
	assert.Equal(t, int32(1), callCount.Load())
}

// TestTokenManager_FullLifecycle: кэш → auth error → failed refresh → success → кэш
func TestTokenManager_FullLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Первые 3 запроса (retry первого refresh) падают
		if callCount.Load() <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// 4-й запрос — успех
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "fresh-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "initial-token")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// 1. Старый токен работает
	token, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "initial-token", token)
	assert.Equal(t, int32(0), callCount.Load())

	// 2. API вернул 403 → токен инвалидирован
	tm.handleAuthError()
	assert.Empty(t, tm.accessToken)

	// 3. Refresh падает (3 retry → ошибка)
	_, err = tm.getToken(ctx)
	require.Error(t, err)
	assert.Equal(t, int32(3), callCount.Load())

	// 4. Следующий вызов — refresh работает (4-й запрос)
	token, err = tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "fresh-token", token)
	assert.Equal(t, int32(4), callCount.Load())

	// 5. Кэш
	for i := 0; i < 5; i++ {
		token, err := tm.getToken(ctx)
		require.NoError(t, err)
		assert.Equal(t, "fresh-token", token)
	}
	assert.Equal(t, int32(4), callCount.Load())
}

// TestTokenManager_ParallelFailures: 100 горутин, refresh падает, singleflight = 1 вызов refresh.
func TestTokenManager_ParallelFailures(t *testing.T) {
	t.Parallel()

	// Таймаут < failedRefreshDelay чтобы второй refresh не успел
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	const goroutines = 100
	var wg sync.WaitGroup
	errors := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := tm.getToken(ctx)
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	for i := 0; i < goroutines; i++ {
		require.Error(t, errors[i], "goroutine %d should have errored", i)
	}

	// singleflight = 1 вызов refresh(), внутри retry с backoff.
	// За 500ms успевает 1-3 retry (backoff: 200ms, 400ms, 800ms).
	count := callCount.Load()
	assert.GreaterOrEqual(t, count, int32(1), "expected at least 1 HTTP request")
	assert.LessOrEqual(t, count, int32(maxRefreshAttempts), "expected at most %d HTTP requests", maxRefreshAttempts)
}

// TestTokenManager_ParallelMixed: токен инвалидируется ДО старта, все идут на refresh.
func TestTokenManager_ParallelMixed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "mixed-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "start-token")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	tm.handleAuthError()

	const goroutines = 50
	var wg sync.WaitGroup
	tokens := make([]string, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, err := tm.getToken(ctx)
			tokens[idx] = token
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i := 0; i < goroutines; i++ {
		require.NoError(t, errs[i], "goroutine %d error: %v", i, errs[i])
		assert.Equal(t, "mixed-token", tokens[i], "goroutine %d token mismatch", i)
	}

	// singleflight = 1 refresh
	assert.Equal(t, int32(1), callCount.Load())
}

// TestTokenManager_RetryWithBackoff проверяет что retry с backoff
// действительно происходит и в итоге succeeds.
func TestTokenManager_RetryWithBackoff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			// Первые 2 падают, 3-й успех
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "backoff-token"}`))
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	token, err := tm.getToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "backoff-token", token)
	assert.Equal(t, int32(3), callCount.Load())
}

// TestTokenManager_DoubleCheckAfterCooldown проверяет что после cooldown
// делается double-check и лишний refresh не происходит.
func TestTokenManager_DoubleCheckAfterCooldown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := newTestLogger()

	var callCount atomic.Int32
	var mu sync.Mutex
	refreshed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount.Add(1)
		firstCall := !refreshed
		refreshed = true
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if firstCall {
			_, _ = w.Write([]byte(`{"access_token": "first-token"}`))
		} else {
			// Второй refresh не должен произойти
			t.Error("unexpected second refresh call")
		}
	}))
	defer server.Close()

	tm := newTestTokenManager(server.URL, logger, "")
	tm.lastRefresh = time.Now().Add(-time.Hour)

	// Первый refresh
	token1, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "first-token", token1)

	// Второй вызов — кэш, без refresh
	token2, err := tm.getToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "first-token", token2)

	assert.Equal(t, int32(1), callCount.Load())
}
