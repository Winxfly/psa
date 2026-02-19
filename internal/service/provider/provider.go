package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

const cacheSaveTimeout = 10 * time.Second

type ProfessionProvider interface {
	GetAllProfessions(ctx context.Context) ([]domain.Profession, error)
	GetActiveProfessions(ctx context.Context) ([]domain.Profession, error)
	GetProfessionByID(ctx context.Context, id uuid.UUID) (domain.Profession, error)
	GetProfessionByName(ctx context.Context, name string) (domain.Profession, error)
	AddProfession(ctx context.Context, profession domain.Profession) (uuid.UUID, error)
	UpdateProfession(ctx context.Context, profession domain.Profession) error
}

type SessionProvider interface {
	GetLatestScraping(ctx context.Context) (domain.Scraping, error)
}

type StatProvider interface {
	GetLatestStatByProfessionID(ctx context.Context, professionID uuid.UUID) (domain.Stat, error)
}

type SkillsProvider interface {
	GetFormalSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]domain.Skill, error)
	GetExtractedSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]domain.Skill, error)
}

type CacheProvider interface {
	SaveProfessionData(ctx context.Context, data *domain.ProfessionDetail) error
	GetProfessionData(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionDetail, error)
}

type Provider struct {
	professionProvider ProfessionProvider
	sessionProvider    SessionProvider
	statProvider       StatProvider
	skillsProvider     SkillsProvider
	cache              CacheProvider
}

func New(
	professionProvider ProfessionProvider,
	sessionProvider SessionProvider,
	statProvider StatProvider,
	skillsProvider SkillsProvider,
	cache CacheProvider,
) *Provider {
	return &Provider{
		professionProvider: professionProvider,
		sessionProvider:    sessionProvider,
		statProvider:       statProvider,
		skillsProvider:     skillsProvider,
		cache:              cache,
	}
}

func (p *Provider) ActiveProfessions(ctx context.Context) ([]domain.ActiveProfession, error) {
	const op = "service.provider.ActiveProfessions"
	log := loggerctx.FromContext(ctx).With("op", op)

	professions, err := p.professionProvider.GetActiveProfessions(ctx)
	if err != nil {
		log.Error("get_active_professions_failed", slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	response := make([]domain.ActiveProfession, len(professions))
	for i, prof := range professions {
		response[i] = domain.ActiveProfession{
			ID:           prof.ID,
			Name:         prof.Name,
			VacancyQuery: prof.VacancyQuery,
		}
	}

	log.Debug("active_professions_loaded", "count", len(response))

	return response, nil
}

func (p *Provider) AllProfessions(ctx context.Context) ([]domain.Profession, error) {
	const op = "service.provider.AllProfessions"
	log := loggerctx.FromContext(ctx).With("op", op)

	professions, err := p.professionProvider.GetAllProfessions(ctx)
	if err != nil {
		log.Error("get_all_professions_failed", slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("all_professions_loaded", "count", len(professions))

	return professions, nil
}

func (p *Provider) ProfessionByID(ctx context.Context, id uuid.UUID) (*domain.Profession, error) {
	const op = "service.provider.ProfessionByID"
	log := loggerctx.FromContext(ctx).With("op", op)

	profession, err := p.professionProvider.GetProfessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrProfessionNotFound) {
			log.Warn("profession_not_found", "profession_id", id)
			return nil, domain.ErrProfessionNotFound
		}

		log.Error("get_profession_failed", "profession_id", id, slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("profession_found", "profession_id", id)

	return &profession, nil
}

func (p *Provider) CreateProfession(ctx context.Context, profession domain.Profession) (uuid.UUID, error) {
	const op = "service.provider.CreateProfession"
	log := loggerctx.FromContext(ctx).With("op", op)

	if err := validateProfessionInput(profession); err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	profession.IsActive = true

	id, err := p.professionProvider.AddProfession(ctx, profession)
	if err != nil {
		if errors.Is(err, domain.ErrProfessionAlreadyExists) {
			log.Warn("profession_already_exists", "profession_name", profession.Name)
			return uuid.Nil, domain.ErrProfessionAlreadyExists
		}

		log.Error("create_profession_failed", "profession_name", profession.Name, slogx.Err(err))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("profession_created", "profession_id", id)

	return id, nil
}

func (p *Provider) ChangeProfession(ctx context.Context, profession domain.Profession) error {
	const op = "service.provider.ChangeProfession"
	log := loggerctx.FromContext(ctx).With("op", op)

	if profession.ID == uuid.Nil {
		return domain.ErrInvalidProfessionID
	}
	if err := validateProfessionInput(profession); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	err := p.professionProvider.UpdateProfession(ctx, profession)
	if err != nil {
		if errors.Is(err, domain.ErrProfessionAlreadyExists) {
			log.Warn("profession_already_exists", "profession_id", profession.ID)
			return domain.ErrProfessionAlreadyExists
		}

		log.Error("update_profession_failed", "profession_id", profession.ID, slogx.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("profession_updated", "profession_id", profession.ID)

	return nil
}

func (p *Provider) ProfessionSkills(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionDetail, error) {
	const op = "service.provider.ProfessionSkills"
	log := loggerctx.FromContext(ctx).With("op", op)

	if p.cache != nil {
		cached, err := p.cache.GetProfessionData(ctx, professionID)
		if err != nil {
			log.Warn("cache_get_failed", "profession_id", professionID, slogx.Err(err))
		} else if cached != nil {
			log.Debug("cache_hit", "profession_id", professionID)
			return cached, nil
		} else {
			log.Debug("cache_miss", "profession_id", professionID)
		}
	}

	profession, err := p.professionProvider.GetProfessionByID(ctx, professionID)
	if errors.Is(err, domain.ErrProfessionNotFound) {
		return nil, domain.ErrProfessionNotFound
	} else if err != nil {
		log.Error("get_profession_failed", "profession_id", professionID, slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	latestScraping, err := p.sessionProvider.GetLatestScraping(ctx)
	if err != nil {
		log.Error("get_latest_scraping_failed", "profession_id", professionID, slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stat, err := p.statProvider.GetLatestStatByProfessionID(ctx, professionID)
	if err != nil {
		log.Error("get_latest_stat_failed", "profession_id", professionID, slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	formalSkills, err := p.skillsProvider.GetFormalSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		log.Error("get_formal_skills_failed", "profession_id", professionID, slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	extractedSkills, err := p.skillsProvider.GetExtractedSkillsByProfessionAndDate(ctx, professionID, latestScraping.ID)
	if err != nil {
		log.Error("get_extracted_skills_failed", "profession_id", professionID, slogx.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	response := &domain.ProfessionDetail{
		ProfessionID:    professionID,
		ProfessionName:  profession.Name,
		ScrapedAt:       latestScraping.ScrapedAt.Format(time.RFC3339),
		VacancyCount:    stat.VacancyCount,
		FormalSkills:    p.transformAndSortSkills(formalSkills),
		ExtractedSkills: p.transformAndSortSkills(extractedSkills),
	}

	if p.cache != nil {
		dataCopy := *response
		parentLog := log

		go func(l *slog.Logger, data domain.ProfessionDetail) {
			cacheCtx, cancel := context.WithTimeout(context.Background(), cacheSaveTimeout)
			defer cancel()

			cacheLog := l.With("async", "cache_save")

			if err := p.cache.SaveProfessionData(cacheCtx, &data); err != nil {
				cacheLog.Error("cache_save_failed", "profession_id", data.ProfessionID, slogx.Err(err))
			} else {
				cacheLog.Debug("cache_saved", "profession_id", data.ProfessionID)
			}
		}(parentLog, dataCopy)
	}

	log.Debug("profession_skills_loaded", "profession_id", professionID)

	return response, nil
}

func (p *Provider) transformAndSortSkills(skills []domain.Skill) []domain.SkillResponse {
	resp := make([]domain.SkillResponse, len(skills))
	for i, s := range skills {
		resp[i] = domain.SkillResponse{
			Skill: s.Skill,
			Count: s.Count,
		}
	}

	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Count > resp[j].Count
	})

	return resp
}

func validateProfessionInput(profession domain.Profession) error {
	if strings.TrimSpace(profession.Name) == "" {
		return domain.ErrInvalidProfessionName
	}
	if strings.TrimSpace(profession.VacancyQuery) == "" {
		return domain.ErrInvalidProfessionQuery
	}

	return nil
}
