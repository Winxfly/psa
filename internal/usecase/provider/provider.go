package provider

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"psa/internal/entity"
	"sort"
	"time"
)

type ProfessionProvider interface {
	GetActiveProfessions(ctx context.Context) ([]entity.Profession, error)
	GetProfessionByID(ctx context.Context, id uuid.UUID) (entity.Profession, error)
}

type SessionProvider interface {
	GetLatestScraping(ctx context.Context) (entity.Scraping, error)
}

type StatProvider interface {
	GetLatestStatByProfessionID(ctx context.Context, professionID uuid.UUID) (entity.Stat, error)
}

type SkillsProvider interface {
	GetFormalSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error)
	GetExtractedSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error)
}

type CacheProvider interface {
	SaveProfessionData(ctx context.Context, data *entity.ProfessionDetail) error
	GetProfessionData(ctx context.Context, professionID uuid.UUID) (*entity.ProfessionDetail, error)
}

type Provider struct {
	log                *slog.Logger
	professionProvider ProfessionProvider
	sessionProvider    SessionProvider
	statProvider       StatProvider
	skillsProvider     SkillsProvider
	cache              CacheProvider
}

func New(
	log *slog.Logger,
	professionProvider ProfessionProvider,
	sessionProvider SessionProvider,
	statProvider StatProvider,
	skillsProvider SkillsProvider,
	cache CacheProvider,
) *Provider {
	return &Provider{
		log:                log,
		professionProvider: professionProvider,
		sessionProvider:    sessionProvider,
		statProvider:       statProvider,
		skillsProvider:     skillsProvider,
		cache:              cache,
	}
}

type ProfessionResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VacancyQuery string    `json:"vacancy_query"`
}

func (p *Provider) ActiveProfessions(ctx context.Context) ([]ProfessionResponse, error) {
	const op = "internal.usecase.provider.ActiveProfessions"

	professions, err := p.professionProvider.GetActiveProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	response := make([]ProfessionResponse, len(professions))
	for i, p := range professions {
		response[i] = ProfessionResponse{
			ID:           p.ID,
			Name:         p.Name,
			VacancyQuery: p.VacancyQuery,
		}
	}

	return response, nil
}

func (p *Provider) ProfessionSkills(ctx context.Context, professionID uuid.UUID) (*entity.ProfessionDetail, error) {
	const op = "internal.usecase.provider.ProfessionSkills"

	if p.cache != nil {
		cached, err := p.cache.GetProfessionData(ctx, professionID)
		if err != nil {
			p.log.Info("Cache empty")
		} else if cached != nil {
			p.log.Info("Cache hit")
			return cached, nil
		}
	}

	profession, err := p.professionProvider.GetProfessionByID(ctx, professionID)
	if err != nil {
		p.log.Error("Failed to get profession", "id", professionID)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	latestScraping, err := p.sessionProvider.GetLatestScraping(ctx)
	if err != nil {
		p.log.Error("Failed to get latest scraping", "id", professionID)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	vacancyCount := int32(0)
	stat, err := p.statProvider.GetLatestStatByProfessionID(ctx, professionID)
	if err != nil {
		p.log.Error("Failed to get latest stat", "id", professionID)
		return nil, fmt.Errorf("%s: %w", op, err)
	} else {
		vacancyCount = stat.VacancyCount
	}

	formalSkills, err := p.skillsProvider.GetFormalSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		p.log.Error("Failed to get formal skills", "id", professionID)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	extractedSkills, err := p.skillsProvider.GetExtractedSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		p.log.Error("Failed to get extracted skills", "id", professionID)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	response := &entity.ProfessionDetail{
		ProfessionID:    professionID,
		ProfessionName:  profession.Name,
		ScrapedAt:       latestScraping.ScrapedAt.Format(time.RFC3339),
		VacancyCount:    vacancyCount,
		FormalSkills:    p.transformSkillsSort(formalSkills),
		ExtractedSkills: p.transformSkillsSort(extractedSkills),
	}

	if p.cache != nil {
		go func() {
			ctx := context.Background()
			_ = p.cache.SaveProfessionData(ctx, response)
			p.log.Info("Cache saved")
		}()
	}

	return response, nil
}

func (p *Provider) transformSkillsSort(skills []entity.Skill) []entity.SkillResponse {
	resp := make([]entity.SkillResponse, len(skills))
	for i, s := range skills {
		resp[i] = entity.SkillResponse{
			Skill: s.Skill,
			Count: s.Count,
		}
	}

	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Count > resp[j].Count
	})

	return resp
}
