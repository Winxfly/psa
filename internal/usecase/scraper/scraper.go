package scraper

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"psa/internal/entity"
	"psa/internal/usecase"
	"psa/internal/usecase/extractor"
)

const (
	ngram = 3
	area  = "113" // Russia
)

type Scraper struct {
	hhAPI usecase.HHAPI
	repo  usecase.Repository
	log   *slog.Logger
}

func New(
	hhAPI usecase.HHAPI,
	repo usecase.Repository,
	log *slog.Logger,
) *Scraper {
	return &Scraper{
		hhAPI: hhAPI,
		repo:  repo,
		log:   log,
	}
}

func (s *Scraper) ProcessActiveProfessions(ctx context.Context) error {
	const op = "usecase.scraper.ProcessActiveProfessions"

	s.log.Info("Starting active professions processing")

	professions, err := s.repo.GetActiveProfessions(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	sessionID, err := s.repo.CreateScrapingSession(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	s.log.Info("Created scraping session", "session_id", sessionID)

	for _, profession := range professions {
		if err := s.processProfession(ctx, profession, sessionID); err != nil {
			s.log.Error("Failed to process profession", "profession", profession.Name, "error", err)
			continue
		}
	}

	s.log.Info("Completed processing all active professions")
	return nil
}

func (s *Scraper) processProfession(ctx context.Context, profession entity.Profession, sessionID uuid.UUID) error {
	s.log.Info("Processing profession", "profession", profession.Name, "query", profession.VacancyQuery)

	vacancyData, err := s.hhAPI.DataProfession(ctx, profession.VacancyQuery, area)
	if err != nil {
		return fmt.Errorf("fetch vacancy data: %w", err)
	}

	s.log.Info("Successfully fetched vacancy data", "profession", profession.Name, "count", len(vacancyData))

	if err := s.repo.SaveStat(ctx, sessionID, profession.ID, len(vacancyData)); err != nil {
		s.log.Error("Failed to save stat", "profession", profession.Name, "error", err)
	}

	formalSkills := s.aggregateFormalSkills(vacancyData)
	if err := s.repo.SaveFormalSkills(ctx, sessionID, profession.ID, formalSkills); err != nil {
		s.log.Error("Failed to save formal skills", "profession", profession.Name, "error", err)
	} else {
		s.log.Info("Saved formal skills", "profession", profession.Name, "skill_count", len(formalSkills))
	}

	extractedSkills := s.extractSkillsFromText(vacancyData, formalSkills)
	if err := s.repo.SaveExtractedSkills(ctx, sessionID, profession.ID, extractedSkills); err != nil {
		s.log.Error("Failed to save extracted skills", "profession", profession.Name, "error", err)
	} else {
		s.log.Info("Saved extracted skills", "profession", profession.Name, "skill_count", len(extractedSkills))
	}

	return nil
}

func (s *Scraper) aggregateFormalSkills(data []entity.VacancyData) map[string]int {
	skills := make(map[string]int)
	for _, d := range data {
		for _, skill := range d.Skills {
			skills[skill]++
		}
	}
	return skills
}

func (s *Scraper) extractSkillsFromText(data []entity.VacancyData, whiteList map[string]int) map[string]int {
	result := make(map[string]int)
	for _, d := range data {
		extracted, err := extractor.ExtractSkills(d.Description, whiteList, ngram)
		if err != nil {
			s.log.Error("Failed to extract skills from text", "error", err)
			continue
		}

		for skill, count := range extracted {
			result[skill] += count
		}
	}
	return result
}
