package scraper

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"psa/internal/entity"
	"sort"
	"time"
)

const (
	ngram = 3
	area  = "113" // Russia
)

type ProfessionProvider interface {
	GetActiveProfessions(ctx context.Context) ([]entity.Profession, error)
}

type SessionProvider interface {
	CreateScrapingSession(ctx context.Context) (uuid.UUID, error)
}

type SkillsProvider interface {
	SaveFormalSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error
	SaveExtractedSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error
}

type StatProvider interface {
	SaveStat(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, vacancyCount int) error
}

type VacancyProvider interface {
	DataProfession(ctx context.Context, query, area string) ([]entity.VacancyData, error)
}

type Extractor interface {
	ExtractSkills(text string, whiteList map[string]int, maxNgram int) (map[string]int, error)
}

type CacheProvider interface {
	SaveProfessionData(ctx context.Context, data *entity.ProfessionDetail) error
}

type Scraper struct {
	log                *slog.Logger
	professionProvider ProfessionProvider
	sessionProvider    SessionProvider
	skillsProvider     SkillsProvider
	statProvider       StatProvider
	vacancyProvider    VacancyProvider
	extractor          Extractor
	cache              CacheProvider
}

func New(
	log *slog.Logger,
	professionProvider ProfessionProvider,
	sessionCreator SessionProvider,
	skillSaver SkillsProvider,
	statSaver StatProvider,
	vacancyFetcher VacancyProvider,
	extractor Extractor,
	cache CacheProvider,
) *Scraper {
	return &Scraper{
		log:                log,
		professionProvider: professionProvider,
		sessionProvider:    sessionCreator,
		skillsProvider:     skillSaver,
		statProvider:       statSaver,
		vacancyProvider:    vacancyFetcher,
		extractor:          extractor,
		cache:              cache,
	}
}

func (s *Scraper) ProcessActiveProfessions(ctx context.Context, saveToDB bool) error {
	const op = "usecase.scraper.ProcessActiveProfessions"

	s.log.Info("Starting active professions processing", "saveToDB", saveToDB)

	professions, err := s.professionProvider.GetActiveProfessions(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	var sessionID uuid.UUID
	if saveToDB {
		sessionID, err = s.sessionProvider.CreateScrapingSession(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		s.log.Info("Created DB session", "session_id", sessionID)
	} else {
		sessionID = uuid.New()
		s.log.Info("Using temporary session", "session_id", sessionID)
	}

	for _, profession := range professions {
		if err := s.processProfession(ctx, profession, sessionID, saveToDB); err != nil {
			s.log.Error("Failed to process profession", "profession", profession.Name, "error", err)
			continue
		}
	}

	s.log.Info("Completed processing all active professions")
	return nil
}

func (s *Scraper) processProfession(ctx context.Context, profession entity.Profession, sessionID uuid.UUID, saveToDB bool) error {
	s.log.Info("Processing profession", "profession", profession.Name, "query", profession.VacancyQuery)

	vacancyData, err := s.vacancyProvider.DataProfession(ctx, profession.VacancyQuery, area)
	if err != nil {
		s.log.Error("Failed to get profession data", "profession", profession.Name, "error", err)
		return fmt.Errorf("fetch vacancy data: %w", err)
	}

	s.log.Info("Successfully fetched vacancy data", "profession", profession.Name, "count", len(vacancyData))

	if saveToDB {
		if err := s.statProvider.SaveStat(ctx, sessionID, profession.ID, len(vacancyData)); err != nil {
			s.log.Error("Failed to save stat", "profession", profession.Name, "error", err)
		}

		formalSkills := s.aggregateFormalSkills(vacancyData)
		if err := s.skillsProvider.SaveFormalSkills(ctx, sessionID, profession.ID, formalSkills); err != nil {
			s.log.Error("Failed to save formal skills", "profession", profession.Name, "error", err)
		} else {
			s.log.Info("Saved formal skills", "profession", profession.Name, "skill_count", len(formalSkills))
		}

		extractedSkills := s.extractSkillsFromText(vacancyData, formalSkills)
		if err := s.skillsProvider.SaveExtractedSkills(ctx, sessionID, profession.ID, extractedSkills); err != nil {
			s.log.Error("Failed to save extracted skills", "profession", profession.Name, "error", err)
		} else {
			s.log.Info("Saved extracted skills", "profession", profession.Name, "skill_count", len(extractedSkills))
		}
	}

	if err := s.saveToCache(ctx, profession, vacancyData); err != nil {
		s.log.Error("Failed to save to cache", "profession", profession.Name, "error", err)
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
		extracted, err := s.extractor.ExtractSkills(d.Description, whiteList, ngram)
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

func (s *Scraper) transformSkillsSort(skills map[string]int) []entity.SkillResponse {
	result := make([]entity.SkillResponse, 0, len(skills))
	for skill, count := range skills {
		result = append(result, entity.SkillResponse{
			Skill: skill,
			Count: int32(count),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}

func (s *Scraper) saveToCache(ctx context.Context, profession entity.Profession, vacancyData []entity.VacancyData) error {
	if s.cache == nil {
		return nil
	}

	formalSkills := s.aggregateFormalSkills(vacancyData)
	extractedSkills := s.extractSkillsFromText(vacancyData, formalSkills)

	cacheData := &entity.ProfessionDetail{
		ProfessionID:    profession.ID,
		ProfessionName:  profession.Name,
		ScrapedAt:       time.Now().Format(time.RFC3339),
		VacancyCount:    int32(len(vacancyData)),
		FormalSkills:    s.transformSkillsSort(formalSkills),
		ExtractedSkills: s.transformSkillsSort(extractedSkills),
	}

	return s.cache.SaveProfessionData(ctx, cacheData)
}
