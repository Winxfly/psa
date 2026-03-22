package hh

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"psa/internal/config"
	"psa/internal/domain"
)

var (
	hasLetterOrDigit = regexp.MustCompile(`[\p{L}\p{N}]`)
)

type professionFetcher interface {
	fetchDataProfession(ctx context.Context, query, area string) (professionData, error)
}

type Adapter struct {
	fetcher professionFetcher
}

func NewAdapter(cfg *config.Config, logger *slog.Logger) *Adapter {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			MaxConnsPerHost:     100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	tokenManager := newTokenManager(cfg.HHAuth, logger)

	client := newClient(cfg, logger, httpClient, tokenManager)

	return &Adapter{
		fetcher: client,
	}
}

func NewAdapterWithClient(client professionFetcher) *Adapter {
	return &Adapter{
		fetcher: client,
	}
}

func (a *Adapter) FetchDataProfession(ctx context.Context, query, area string) ([]domain.VacancyData, int, error) {
	profData, err := a.fetcher.fetchDataProfession(ctx, query, area)
	if err != nil {
		return nil, 0, err
	}

	result := make([]domain.VacancyData, 0, len(profData.Vacancies))

	for _, item := range profData.Vacancies {
		v := domain.VacancyData{
			Skills:      make([]string, 0),
			Description: item.Description,
		}

		for _, skill := range item.KeySkills {
			name := strings.TrimSpace(strings.ToLower(skill.Name))
			if !hasLetterOrDigit.MatchString(name) {
				continue
			}

			v.Skills = append(v.Skills, name)
		}

		result = append(result, v)
	}

	return result, profData.TotalFound, nil
}
