package provider

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/entity"
	"psa/internal/usecase"
	"sort"
	"time"
)

type Provider struct {
	repo usecase.Repository
}

func New(repo usecase.Repository) *Provider {
	return &Provider{repo: repo}
}

type ProfessionResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VacancyQuery string    `json:"vacancy_query"`
}

type SkillResponse struct {
	Skill string `json:"skill"`
	Count int32  `json:"count"`
}

type ProfessionSkillResponse struct {
	ProfessionID    uuid.UUID       `json:"profession_id"`
	ProfessionName  string          `json:"profession_name"`
	ScrapedAt       string          `json:"scraped_at"`
	VacancyCount    int32           `json:"vacancy_count"`
	FormalSkills    []SkillResponse `json:"formal_skills"`
	ExtractedSkills []SkillResponse `json:"extracted_skills"`
}

func (p *Provider) ActiveProfessions(ctx context.Context) ([]ProfessionResponse, error) {
	const op = "internal.usecase.provider.ActiveProfessions"

	professions, err := p.repo.GetActiveProfessions(ctx)
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

func (p *Provider) ProfessionSkills(ctx context.Context, professionID uuid.UUID) (*ProfessionSkillResponse, error) {
	const op = "internal.usecase.provider.ProfessionSkills"

	profession, err := p.repo.GetProfessionByID(ctx, professionID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	latestScraping, err := p.repo.GetLatestScraping(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	vacancyCount := int32(0)
	stat, err := p.repo.GetLatestStatByProfessionID(ctx, professionID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	} else {
		vacancyCount = stat.VacancyCount
	}

	formalSkills, err := p.repo.GetFormalSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	extractedSkills, err := p.repo.GetExtractedSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &ProfessionSkillResponse{
		ProfessionID:    professionID,
		ProfessionName:  profession.Name,
		ScrapedAt:       latestScraping.ScrapedAt.Format(time.RFC3339),
		VacancyCount:    vacancyCount,
		FormalSkills:    p.transformSkillsSort(formalSkills),
		ExtractedSkills: p.transformSkillsSort(extractedSkills),
	}, nil
}

func (p *Provider) transformSkillsSort(skills []entity.Skill) []SkillResponse {
	resp := make([]SkillResponse, len(skills))
	for i, s := range skills {
		resp[i] = SkillResponse{
			Skill: s.Skill,
			Count: s.Count,
		}
	}

	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Count > resp[j].Count
	})

	return resp
}
