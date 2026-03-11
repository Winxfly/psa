package hh

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"

	"psa/internal/config"
)

const (
	perPage = 100
	baseURL = "https://api.hh.ru/vacancies"

	// HH API allows access to max 2000 vacancies (20 pages * 100 per page)
	maxVacancies = 2000

	workers     = 25
	pageWorkers = 5

	rps = 35
)

type tokenProvider interface {
	getToken(ctx context.Context) (string, error)
	handleAuthError()
}

type client struct {
	cfg     *config.Config
	logger  *slog.Logger
	hClient *http.Client
	limiter *rate.Limiter
	token   tokenProvider
}

func newClient(cfg *config.Config, logger *slog.Logger, hClient *http.Client, token tokenProvider) *client {
	return &client{
		cfg:     cfg,
		logger:  logger,
		hClient: hClient,
		limiter: rate.NewLimiter(rps, rps),
		token:   token,
	}
}

func (c *client) setHeaders(req *http.Request) error {
	accessToken, err := c.token.getToken(req.Context())
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("User-Agent", c.cfg.HHAuth.UserAgent)
	req.Header.Set("Accept", "application/json")

	return nil
}

func (c *client) calculateWait(attempt int) time.Duration {
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

func (c *client) doRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	const op = "integration.hh.hClient.doRequestWithRetry"

	startTime := time.Now()

	for attempt := 0; attempt < c.cfg.HHRetry.MaxAttempts; attempt++ {
		if time.Since(startTime) > c.cfg.HHRetry.MaxTotalTime {
			return nil, fmt.Errorf("%s: retry timeout exceeded", op)
		}

		reqClone := req.Clone(ctx)

		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("%s: rate limiter: %w", op, err)
		}

		if err := c.setHeaders(reqClone); err != nil {
			return nil, fmt.Errorf("%s: set headers: %w", op, err)
		}

		resp, err := c.hClient.Do(reqClone)
		if err != nil {
			if attempt < c.cfg.HHRetry.MaxAttempts-1 {
				wait := c.calculateWait(attempt)

				c.logger.WarnContext(ctx, op,
					"event", "http_retry",
					"attempt", attempt+1,
					"wait_duration", wait,
					"error", err,
				)
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, fmt.Errorf("%s: do request: %w", op, err)
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		if resp.StatusCode == http.StatusForbidden {
			c.token.handleAuthError()

			c.logger.WarnContext(ctx, op,
				"event", "auth_forbidden",
				"attempt", attempt+1,
			)
		}

		if isRetryable(resp.StatusCode) {
			resp.Body.Close()

			if attempt < c.cfg.HHRetry.MaxAttempts-1 {
				wait := c.calculateWait(attempt)

				c.logger.WarnContext(ctx, op,
					"event", "http_retry_status",
					"attempt", attempt+1,
					"status_code", resp.StatusCode,
					"wait_duration", wait,
				)

				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}

		resp.Body.Close()
		return nil, fmt.Errorf("%s: unexpected status %d", op, resp.StatusCode)
	}

	return nil, fmt.Errorf("%s: max retry attempts exceeded", op)
}

func (c *client) fetchMeta(ctx context.Context, query, area string) (metadata, error) {
	const op = "integration.hh.hClient.fetchMeta"

	params := url.Values{
		"text":         []string{query},
		"search_field": []string{"name"},
		"per_page":     []string{fmt.Sprintf("%d", perPage)},
		"page":         []string{"0"},
		"area":         []string{area},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return metadata{}, fmt.Errorf("%s: build request: %w", op, err)
	}

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return metadata{}, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	var meta metadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return metadata{}, fmt.Errorf("%s: decode response: %w", op, err)
	}

	if meta.Found == 0 {
		c.logger.WarnContext(ctx, op, "event", "vacancies.not_found", "query", query)
		return metadata{}, fmt.Errorf("%s: vacancies not found", op)
	}

	return meta, nil
}

func (c *client) fetchIDsFromPage(ctx context.Context, page int, query, area string) ([]string, error) {
	const op = "integration.hh.hClient.fetchIDsFromPage"

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

	var ids vacancyIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", op, err)
	}

	for _, id := range ids.Items {
		result = append(result, id.ID)
	}

	return result, nil
}

func (c *client) fetchIDsVacancies(ctx context.Context, meta metadata, query, area string) ([]string, error) {
	const op = "integration.hh.hClient.fetchIDsVacancies"

	lenFound := 0
	if meta.Found > maxVacancies {
		lenFound = maxVacancies
	} else {
		lenFound = meta.Found
	}

	ids := make([]string, 0, lenFound)

	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, pageWorkers)

	for i := 0; i < meta.Pages; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			temp, err := c.fetchIDsFromPage(ctx, i, query, area)
			if err != nil {
				c.logger.ErrorContext(ctx, op, "event", "fetch_ids_failed", "page", i, "query", query, "error", err)
				return
			}

			mu.Lock()
			ids = append(ids, temp...)
			mu.Unlock()
		}(i)

	}

	wg.Wait()

	return ids, nil
}

func (c *client) fetchDataVacancy(ctx context.Context, id string) (vacancyResponse, error) {
	const op = "integration.hh.hClient.fetchDataVacancy"

	link := fmt.Sprintf("%s/%s", baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return vacancyResponse{}, fmt.Errorf("%s: build request: %w", op, err)
	}

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return vacancyResponse{}, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	var data vacancyResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return vacancyResponse{}, fmt.Errorf("%s: decode response: %w", op, err)
	}

	return data, nil
}

func (c *client) fetchDataVacancies(ctx context.Context, ids []string) ([]vacancyResponse, error) {
	const op = "integration.hh.hClient.fetchDataVacancies"

	jobs := make(chan string, workers)
	data := make(chan vacancyResponse, workers)
	result := make([]vacancyResponse, 0, len(ids))

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for id := range jobs {
				vac, err := c.fetchDataVacancy(ctx, id)
				if err != nil {
					c.logger.ErrorContext(ctx, op, "event", "fetch_vacancy_failed", "vacancy_id", id, "error", err)
					continue
				}

				select {
				case data <- vac:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)

		for _, id := range ids {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(data)
	}()

	for v := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result = append(result, v)
		}
	}

	return result, nil
}

func (c *client) fetchDataProfession(ctx context.Context, query, area string) (professionData, error) {
	const op = "integration.hh.hClient.fetchDataProfession"

	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	if query == "" {
		return professionData{}, fmt.Errorf("%s: query cannot be empty", op)
	}
	if area == "" {
		return professionData{}, fmt.Errorf("%s: area cannot be empty", op)
	}

	meta, err := c.fetchMeta(ctx, query, area)
	if err != nil {
		return professionData{}, fmt.Errorf("%s: %w", op, err)
	}

	ids, err := c.fetchIDsVacancies(ctx, meta, query, area)
	if err != nil {
		return professionData{}, fmt.Errorf("%s: %w", op, err)
	}

	data, err := c.fetchDataVacancies(ctx, ids)
	if err != nil {
		return professionData{}, fmt.Errorf("%s: %w", op, err)
	}

	c.logger.InfoContext(ctx, op, "event", "data_collected", "query", query, "vacancies_count", len(data), "total_found", meta.Found)

	return professionData{
		Vacancies:  data,
		TotalFound: meta.Found,
	}, nil
}

func isRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return true
	default:
		return statusCode >= 500 && statusCode < 600
	}
}
