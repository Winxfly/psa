package hh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"psa/internal/config"
)

const (
	defaultBaseURL     = "https://api.hh.ru/token"
	minRefreshInterval = 5 * time.Minute
	failedRefreshDelay = 1 * time.Second
	grantType          = "client_credentials"

	maxRefreshAttempts = 3
	initialBackoff     = 200 * time.Millisecond
	maxBackoff         = 2 * time.Second
)

type tokenManager struct {
	mu           sync.Mutex
	sf           singleflight.Group
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	logger       *slog.Logger

	accessToken       string
	lastRefresh       time.Time
	lastFailedRefresh time.Time
}

func newTokenManager(cfg config.HHAuth, logger *slog.Logger) *tokenManager {
	tm := &tokenManager{
		baseURL:      defaultBaseURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
	}

	if cfg.AccessToken != "" {
		tm.accessToken = cfg.AccessToken
	}

	tm.lastRefresh = time.Now().Add(-minRefreshInterval - time.Minute)

	return tm
}

func (tm *tokenManager) getToken(ctx context.Context) (string, error) {
	// fast path: return cached token without locking/refresh
	tm.mu.Lock()
	if tm.accessToken != "" {
		token := tm.accessToken
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	// singleflight
	v, err, _ := tm.sf.Do("refresh_token", func() (interface{}, error) {
		return tm.refresh(ctx)
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

func (tm *tokenManager) refresh(ctx context.Context) (string, error) {
	const op = "integration.hh.token.refresh"

	// cooldown
	tm.mu.Lock()
	wait := time.Duration(0)

	if w := minRefreshInterval - time.Since(tm.lastRefresh); w > 0 {
		wait = w
	}
	if w := failedRefreshDelay - time.Since(tm.lastFailedRefresh); w > wait {
		wait = w
	}
	tm.mu.Unlock()

	if wait > 0 {
		tm.logger.InfoContext(ctx, op, "event", "refresh_wait", "wait_duration", wait)

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// double-check
	tm.mu.Lock()
	if tm.accessToken != "" {
		token := tm.accessToken
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	var lastErr error

	for attempt := 0; attempt < maxRefreshAttempts; attempt++ {
		if attempt > 0 {
			wait := calculateBackoff(attempt)

			tm.logger.WarnContext(ctx, op,
				"event", "refresh_retry",
				"attempt", attempt+1,
				"wait_duration", wait,
				"error", lastErr,
			)

			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		token, err := tm.doRefreshRequest(ctx)
		if err == nil {
			return token, nil
		}

		lastErr = err
	}

	tm.markFailed()

	tm.logger.ErrorContext(ctx, op,
		"event", "refresh_failed",
		"attempts", maxRefreshAttempts,
		"error", lastErr,
	)

	return "", fmt.Errorf("%s: refresh failed after retries: %w", op, lastErr)
}

func (tm *tokenManager) doRefreshRequest(ctx context.Context) (string, error) {
	const op = "integration.hh.token.doRefreshRequest"

	tm.logger.InfoContext(ctx, op, "event", "refresh_token")

	data := url.Values{}
	data.Set("grant_type", grantType)
	data.Set("client_id", tm.clientID)
	data.Set("client_secret", tm.clientSecret)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tm.baseURL,
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("%s: build request: %w", op, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: do request: %w", op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: unexpected status %d", op, resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("%s: decode: %w", op, err)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("%s: empty token", op)
	}

	tm.mu.Lock()
	tm.accessToken = result.AccessToken
	tm.lastRefresh = time.Now()
	tm.lastFailedRefresh = time.Time{}
	tm.mu.Unlock()

	tm.logger.InfoContext(ctx, op, "event", "refresh_success")

	return result.AccessToken, nil
}

func (tm *tokenManager) handleAuthError() {
	const op = "integration.hh.token.handleAuthError"

	tm.mu.Lock()
	tm.accessToken = ""
	tm.mu.Unlock()

	tm.logger.WarnContext(context.Background(), op, "event", "token_cleared")
}

func (tm *tokenManager) markFailed() {
	tm.mu.Lock()
	tm.lastFailedRefresh = time.Now()
	tm.mu.Unlock()
}

func calculateBackoff(attempt int) time.Duration {
	base := float64(initialBackoff) * math.Pow(2, float64(attempt-1))
	if base > float64(maxBackoff) {
		base = float64(maxBackoff)
	}

	// equal jitter
	delay := base/2 + rand.Float64()*(base/2)

	return time.Duration(delay)
}
