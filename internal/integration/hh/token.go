package hh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"psa/internal/config"
)

const (
	defaultBaseURL     = "https://api.hh.ru/token"
	minRefreshInterval = 5 * time.Minute
	grantType          = "client_credentials"
)

// tokenManager manages HH API tokens with automatic refresh.
type tokenManager struct {
	mu           sync.Mutex
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	logger       *slog.Logger

	accessToken   string
	lastRefresh   time.Time
	refreshFailed bool
	refreshing    bool
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
		tm.lastRefresh = time.Now().Add(-minRefreshInterval - time.Minute)
	}

	return tm
}

func (tm *tokenManager) getToken(ctx context.Context) (string, error) {
	const op = "integration.hh.token.getToken"

	for {
		tm.mu.Lock()

		if tm.accessToken != "" && !tm.refreshFailed {
			token := tm.accessToken
			tm.mu.Unlock()
			return token, nil
		}

		if !tm.refreshing {
			tm.refreshing = true
			tm.mu.Unlock()
			break
		}

		tm.mu.Unlock()

		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	waitTime := minRefreshInterval - time.Since(tm.lastRefresh)
	needWait := waitTime > 0

	if needWait {
		tm.logger.InfoContext(ctx, op, "event", "refresh_wait", "wait_duration", waitTime)

		timer := time.NewTimer(waitTime)
		defer timer.Stop()

		select {
		case <-timer.C:
			// continue to refresh
		case <-ctx.Done():
			tm.mu.Lock()
			tm.refreshing = false
			tm.mu.Unlock()
			return "", ctx.Err()
		}
	}

	tm.mu.Lock()
	if tm.accessToken != "" && !tm.refreshFailed {
		token := tm.accessToken
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

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
		tm.mu.Lock()
		tm.refreshFailed = true
		tm.refreshing = false
		tm.mu.Unlock()
		return "", fmt.Errorf("%s: build request: %w", op, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		tm.mu.Lock()
		tm.refreshFailed = true
		tm.refreshing = false
		tm.mu.Unlock()
		return "", fmt.Errorf("%s: do request: %w", op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		tm.mu.Lock()
		tm.refreshFailed = true
		tm.refreshing = false
		tm.mu.Unlock()
		return "", fmt.Errorf("%s: unexpected status code %d", op, resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		tm.mu.Lock()
		tm.refreshFailed = true
		tm.refreshing = false
		tm.mu.Unlock()
		return "", fmt.Errorf("%s: decode response: %w", op, err)
	}

	if result.AccessToken == "" {
		tm.mu.Lock()
		tm.refreshFailed = true
		tm.refreshing = false
		tm.mu.Unlock()
		return "", fmt.Errorf("%s: empty access token", op)
	}

	tm.mu.Lock()
	tm.accessToken = result.AccessToken
	tm.lastRefresh = time.Now()
	tm.refreshFailed = false
	tm.refreshing = false
	tm.mu.Unlock()

	tm.logger.InfoContext(ctx, op, "event", "refresh_success")

	return result.AccessToken, nil
}

func (tm *tokenManager) handleAuthError() {
	const op = "integration.hh.token.handleAuthError"

	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.refreshFailed = true
	tm.logger.WarnContext(context.Background(), op, "event", "token_marked_invalid")
}
