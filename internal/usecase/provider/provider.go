package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"psa/internal/entity"
	"sort"
	"strings"
	"time"
)

type ProfessionProvider interface {
	GetAllProfessions(ctx context.Context) ([]entity.Profession, error)
	GetActiveProfessions(ctx context.Context) ([]entity.Profession, error)
	GetProfessionByID(ctx context.Context, id uuid.UUID) (entity.Profession, error)
	GetProfessionByName(ctx context.Context, name string) (entity.Profession, error)
	AddProfession(ctx context.Context, profession entity.Profession) (uuid.UUID, error)
	UpdateProfession(ctx context.Context, profession entity.Profession) error
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

func (p *Provider) ActiveProfessions(ctx context.Context) ([]entity.ActiveProfession, error) {
	const op = "internal.usecase.provider.ActiveProfessions"

	professions, err := p.professionProvider.GetActiveProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	response := make([]entity.ActiveProfession, len(professions))
	for i, prof := range professions {
		response[i] = entity.ActiveProfession{
			ID:           prof.ID,
			Name:         prof.Name,
			VacancyQuery: prof.VacancyQuery,
		}
	}

	return response, nil
}

func (p *Provider) AllProfessions(ctx context.Context) ([]entity.Profession, error) {
	const op = "internal.usecase.provider.AllProfessions"

	professions, err := p.professionProvider.GetAllProfessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return professions, nil
}

func (p *Provider) ProfessionByID(ctx context.Context, id uuid.UUID) (*entity.Profession, error) {
	const op = "internal.usecase.provider.ProfessionByID"

	profession, err := p.professionProvider.GetProfessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, entity.ErrProfessionNotFound) {
			return nil, entity.ErrProfessionNotFound
		}

		p.log.Error("Failed to get profession", "profession_id", id, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &profession, nil
}

func (p *Provider) CreateProfession(ctx context.Context, profession entity.Profession) (uuid.UUID, error) {
	const op = "internal.usecase.provider.CreateProfession"

	if err := validateProfessionInput(profession); err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	profession.IsActive = true

	id, err := p.professionProvider.AddProfession(ctx, profession)
	if err != nil {
		if errors.Is(err, entity.ErrProfessionAlreadyExists) {
			p.log.Info("Attempt to create duplicate profession", "profession_name", profession.Name)
			return uuid.Nil, entity.ErrProfessionAlreadyExists
		}

		p.log.Error("Failed to add profession", "profession_name", profession.Name, "error", err)

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	p.log.Info("Profession created", "id", id, "profession_name", profession.Name)

	return id, nil
}

func (p *Provider) ChangeProfession(ctx context.Context, profession entity.Profession) error {
	const op = "internal.usecase.provider.ChangeProfession"

	if profession.ID == uuid.Nil {
		return entity.ErrInvalidProfessionID
	}
	if err := validateProfessionInput(profession); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	err := p.professionProvider.UpdateProfession(ctx, profession)
	if err != nil {
		if errors.Is(err, entity.ErrProfessionAlreadyExists) {
			return entity.ErrProfessionAlreadyExists
		}

		p.log.Error("Failed to update profession", "profession_id", profession.ID, "error", err)

		return fmt.Errorf("%s: %w", op, err)
	}

	p.log.Info("Profession updated", "profession_id", profession.ID, "profession_name", profession.Name)

	return nil
}

func (p *Provider) ProfessionSkills(ctx context.Context, professionID uuid.UUID) (*entity.ProfessionDetail, error) {
	const op = "internal.usecase.provider.ProfessionSkills"

	if p.cache != nil {
		cached, err := p.cache.GetProfessionData(ctx, professionID)
		if err != nil {
			p.log.Debug("Cache miss", "profession_id", professionID)
		} else if cached != nil {
			p.log.Debug("Cache hit", "profession_id", professionID)
			return cached, nil
		}
	}

	profession, err := p.professionProvider.GetProfessionByID(ctx, professionID)
	if errors.Is(err, entity.ErrProfessionNotFound) {
		return nil, entity.ErrProfessionNotFound
	} else if err != nil {
		p.log.Error("Failed to get profession", "profession_id", professionID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	latestScraping, err := p.sessionProvider.GetLatestScraping(ctx)
	if err != nil {
		p.log.Error("Failed to get latest scraping", "profession_id", professionID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stat, err := p.statProvider.GetLatestStatByProfessionID(ctx, professionID)
	if err != nil {
		p.log.Error("Failed to get latest stat", "profession_id", professionID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	vacancyCount := stat.VacancyCount

	formalSkills, err := p.skillsProvider.GetFormalSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		p.log.Error("Failed to get formal skills", "profession_id", professionID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	extractedSkills, err := p.skillsProvider.GetExtractedSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		p.log.Error("Failed to get extracted skills", "profession_id", professionID, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	response := &entity.ProfessionDetail{
		ProfessionID:    professionID,
		ProfessionName:  profession.Name,
		ScrapedAt:       latestScraping.ScrapedAt.Format(time.RFC3339),
		VacancyCount:    vacancyCount,
		FormalSkills:    p.transformAndSortSkills(formalSkills),
		ExtractedSkills: p.transformAndSortSkills(extractedSkills),
	}

	if p.cache != nil {
		dataCopy := *response

		go func(data entity.ProfessionDetail) {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := p.cache.SaveProfessionData(cacheCtx, &data); err != nil {
				p.log.Error("Failed to save cache", "error", err)
			} else {
				p.log.Debug("Cache saved", "profession_id", professionID)
			}
		}(dataCopy)
	}

	return response, nil
}

func (p *Provider) transformAndSortSkills(skills []entity.Skill) []entity.SkillResponse {
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

func validateProfessionInput(profession entity.Profession) error {
	if strings.TrimSpace(profession.Name) == "" {
		return entity.ErrInvalidProfessionName
	}
	if strings.TrimSpace(profession.VacancyQuery) == "" {
		return entity.ErrInvalidProfessionQuery
	}

	return nil
}
