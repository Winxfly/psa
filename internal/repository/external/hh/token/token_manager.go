package token

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"psa/internal/config"
	"time"
)

const (
	baseUrl            = "https://api.hh.ru/token"
	minRefreshInterval = 5 * time.Minute
	grantType          = "client_credentials"
)

type TokenManager struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	logger       *slog.Logger

	accessToken   string
	lastRefresh   time.Time
	refreshFailed bool
}

func NewTokenManager(cfg config.HHAuth, logger *slog.Logger) *TokenManager {
	tm := &TokenManager{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
	}

	if cfg.AccessToken != "" {
		tm.accessToken = cfg.AccessToken
		tm.lastRefresh = time.Now().Add(-time.Minute * 6)
	}

	return tm
}

func (tm *TokenManager) GetToken(ctx context.Context) (string, error) {
	const op = "repository.hh.token.GetToken"

	if tm.accessToken != "" && !tm.refreshFailed {
		return tm.accessToken, nil
	}

	if time.Since(tm.lastRefresh) < minRefreshInterval {
		waitTime := minRefreshInterval - time.Since(tm.lastRefresh)
		tm.logger.Info("Waiting refresh token", "wait time", waitTime.String())

		timer := time.NewTimer(waitTime)
		defer timer.Stop()

		select {
		case <-timer.C:
			// continue next step
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	tm.logger.Info("Refreshing HH API access token")

	data := url.Values{}
	data.Set("grant_type", grantType)
	data.Set("client_id", tm.clientID)
	data.Set("client_secret", tm.clientSecret)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseUrl,
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		tm.refreshFailed = true
		return "", fmt.Errorf("%s: %w", op, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		tm.refreshFailed = true
		return "", fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tm.refreshFailed = true
		return "", fmt.Errorf("%s: %w", op, fmt.Errorf("unexpected status code %d", resp.StatusCode))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		tm.refreshFailed = true
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if result.AccessToken == "" {
		tm.refreshFailed = true
		return "", fmt.Errorf("%s: %w", op, fmt.Errorf("empty access token"))
	}

	tm.accessToken = result.AccessToken
	tm.lastRefresh = time.Now()
	tm.refreshFailed = false

	tm.logger.Info("Successfully refreshed HH API access token")

	return result.AccessToken, nil
}

func (tm *TokenManager) HandleAuthError() {
	tm.refreshFailed = true
	tm.logger.Warn("Authentication error detected, token marked for refresh")
}
