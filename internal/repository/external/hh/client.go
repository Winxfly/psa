package hh

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/time/rate"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"psa/internal/config"
	"psa/internal/entity"
	"psa/internal/repository/external/hh/token"
	"psa/pkg/retry"
	"strings"
	"time"
)

const (
	perPage = 100
	baseURL = "https://api.hh.ru/vacancies"
)

type Client struct {
	client  *http.Client
	limiter *rate.Limiter
	cfg     *config.Config
	token   *token.TokenManager
	logger  *slog.Logger
}

func New(logger *slog.Logger, client *http.Client, cfg *config.Config, token *token.TokenManager) *Client {
	return &Client{
		logger:  logger,
		client:  client,
		cfg:     cfg,
		limiter: rate.NewLimiter(5, 5),
		token:   token,
	}
}

type metadata struct {
	Found int `json:"found"`
	Pages int `json:"pages"`
}

type vacancyIDResponse struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
}

type vacancyResponse struct {
	Description string `json:"description"`
	KeySkills   []struct {
		Name string `json:"name"`
	} `json:"key_skills"`
}

func (c *Client) setHeaders(req *http.Request) error {
	accessToken, err := c.token.GetToken(req.Context())
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("User-Agent", c.cfg.HHAuth.UserAgent)
	req.Header.Set("Accept", "application/json")

	return nil
}

func (c *Client) calculateWait(attempt int) time.Duration {
	if attempt == 0 {
		return 0
	}

	baseDelay := float64(c.cfg.HHRetry.InitialDelay) * math.Pow(c.cfg.HHRetry.Multiplier, float64(attempt-1))
	if baseDelay > float64(c.cfg.HHRetry.MaxDelay) {
		baseDelay = float64(c.cfg.HHRetry.MaxDelay)
	}

	// Equal jitter
	delay := baseDelay/2 + (rand.Float64() * baseDelay / 2)

	return time.Duration(delay)
}

func (c *Client) doRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	attempt := 0
	startTime := time.Now()

	for attempt = 0; attempt < c.cfg.HHRetry.MaxAttempts; attempt++ {
		if time.Since(startTime) > c.cfg.HHRetry.MaxTotalTime {
			return nil, fmt.Errorf("retry timeout after %v", time.Since(startTime))
		}

		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}

		if err := c.setHeaders(req); err != nil {
			return nil, fmt.Errorf("set headers: %w", err)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			if attempt < c.cfg.HHRetry.MaxAttempts-1 {
				wait := c.calculateWait(attempt)
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, fmt.Errorf("do request (attempt %d): %w", attempt+1, err)
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		if resp.StatusCode == http.StatusForbidden {
			c.token.HandleAuthError()
		}

		if retry.IsRetryable(resp.StatusCode) && attempt < c.cfg.HHRetry.MaxAttempts-1 {
			resp.Body.Close()
			wait := c.calculateWait(attempt)
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// if last try or no need to repeat
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	return nil, fmt.Errorf("max retry attempts %d exceeded", c.cfg.HHRetry.MaxAttempts)
}

func (c *Client) fetchMeta(ctx context.Context, query, area string) (metadata, error) {
	const op = "repository.hh.vacancy.client.fetchMeta"

	params := url.Values{
		"text":         []string{query},
		"search_field": []string{"name"},
		"per_page":     []string{fmt.Sprintf("%d", perPage)},
		"page":         []string{"0"},
		"area":         []string{area},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return metadata{}, fmt.Errorf("%s: %w", op, err)
	}

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return metadata{}, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return metadata{}, fmt.Errorf("%s: %w", op, err)
	}

	var meta metadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return metadata{}, fmt.Errorf("%s: %w", op, err)
	}

	if meta.Found == 0 {
		c.logger.Error("No vacancies found for query: %s", query)
		return metadata{}, fmt.Errorf("%s: %w", op, err)
	}

	return meta, nil
}

func (c *Client) fetchIDsFromPage(ctx context.Context, page int, query, area string) ([]string, error) {
	const op = "repository.hh.vacancy.client.fetchIDsFromPage"

	result := make([]string, 0)

	params := url.Values{
		"text":         []string{query},
		"search_field": []string{"name"},
		"per_page":     []string{fmt.Sprintf("%d", perPage)},
		"page":         []string{fmt.Sprintf("%d", page)},
		"area":         []string{area},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var ids vacancyIDResponse
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	for _, id := range ids.Items {
		result = append(result, id.ID)
	}

	return result, nil
}

func (c *Client) fetchIDsVacancies(ctx context.Context, meta metadata, query, area string) ([]string, error) {
	ids := make([]string, 0, meta.Found)
	for i := 0; i < meta.Pages; i++ {
		temp, err := c.fetchIDsFromPage(ctx, i, query, area)
		if err != nil {
			c.logger.Error("Failed to fetch ids from page:", i, "query:", query, "error:", err)
			continue
		}
		ids = append(ids, temp...)
	}

	return ids, nil
}

func (c *Client) fetchDataVacancy(ctx context.Context, id string) (entity.VacancyData, error) {
	const op = "repository.hh.vacancy.client.fetchDataVacancy"

	link := fmt.Sprintf("%s/%s", baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return entity.VacancyData{}, fmt.Errorf("%s: %w", op, err)
	}

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return entity.VacancyData{}, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return entity.VacancyData{}, fmt.Errorf("%s: %w", op, err)
	}

	var data vacancyResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return entity.VacancyData{}, fmt.Errorf("%s: %w", op, err)
	}

	var result entity.VacancyData
	for _, item := range data.KeySkills {
		result.Skills = append(result.Skills, strings.ToLower(item.Name))
	}
	result.Description = data.Description

	return result, nil
}

func (c *Client) fetchDataVacancies(ctx context.Context, ids []string) ([]entity.VacancyData, error) {
	result := make([]entity.VacancyData, 0, len(ids))
	for _, id := range ids {
		v, err := c.fetchDataVacancy(ctx, id)
		if err != nil {
			c.logger.Error("Failed to fetch data vacancy for id:", id, "error:", err)
			continue
		}

		result = append(result, v)
	}

	return result, nil
}

func (c *Client) DataProfession(ctx context.Context, query, area string) ([]entity.VacancyData, error) {
	const op = "repository.hh.vacancy.client.DataProfession"

	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()

	if query == "" {
		return nil, fmt.Errorf("query cannot be empty %s", op)
	}
	if area == "" {
		return nil, fmt.Errorf("area cannot be empty %s", op)
	}

	meta, err := c.fetchMeta(ctx, query, area)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	ids, err := c.fetchIDsVacancies(ctx, meta, query, area)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	data, err := c.fetchDataVacancies(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return data, nil
}
