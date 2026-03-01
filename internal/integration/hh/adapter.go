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

type Adapter struct {
	client *client
}

func NewAdapter(cfg *config.Config, logger *slog.Logger) *Adapter {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	tokenManager := newTokenManager(cfg.HHAuth, logger)

	client := newClient(cfg, logger, httpClient, tokenManager)

	return &Adapter{
		client: client,
	}
}

func (a *Adapter) FetchDataProfession(ctx context.Context, query, area string) ([]domain.VacancyData, error) {
	profData, err := a.client.fetchDataProfession(ctx, query, area)
	if err != nil {
		return nil, err
	}

	result := make([]domain.VacancyData, 0, len(profData.Vacancies))

	for _, item := range profData.Vacancies {
		v := domain.VacancyData{
			Skills:      make([]string, 0),
			Description: item.Description,
			TotalFound:  profData.TotalFound,
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

	return result, nil
}
