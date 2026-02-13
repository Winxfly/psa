package scraper

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"psa/internal/entity"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
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
	professionProvider ProfessionProvider
	sessionProvider    SessionProvider
	skillsProvider     SkillsProvider
	statProvider       StatProvider
	vacancyProvider    VacancyProvider
	extractor          Extractor
	cache              CacheProvider
}

func New(
	professionProvider ProfessionProvider,
	sessionCreator SessionProvider,
	skillSaver SkillsProvider,
	statSaver StatProvider,
	vacancyFetcher VacancyProvider,
	extractor Extractor,
	cache CacheProvider,
) *Scraper {
	return &Scraper{
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
	log := loggerctx.FromContext(ctx).With("op", op)

	log.Info("start", "save_to_db", saveToDB)

	professions, err := s.professionProvider.GetActiveProfessions(ctx)
	if err != nil {
		log.Error("get_active_professions_failed", slogx.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("active_professions.loaded", "count", len(professions))

	var sessionID uuid.UUID
	if saveToDB {
		sessionID, err = s.sessionProvider.CreateScrapingSession(ctx)
		if err != nil {
			log.Error("session.create_failed", slogx.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
		log.Info("session.created", "session_id", sessionID)
	} else {
		sessionID = uuid.New()
		log.Info("session.temporary", "session_id", sessionID)
	}

	for _, profession := range professions {
		if err := s.processProfession(ctx, profession, sessionID, saveToDB); err != nil {
			log.Error("profession.process_failed", "profession_id", profession.ID,
				"profession_name", profession.Name, slogx.Err(err))
			continue
		}
	}

	log.Info("completed")
	return nil
}

func (s *Scraper) processProfession(ctx context.Context, profession entity.Profession, sessionID uuid.UUID, saveToDB bool) error {
	const op = "usecase.scraper.processProfession"
	log := loggerctx.FromContext(ctx).With(
		"op", op,
		"profession_id", profession.ID,
		"profession_name", profession.Name,
		"session_id", sessionID,
	)

	log.Debug("start")

	start := time.Now()
	defer func() {
		log.Debug("finished", "duration", time.Since(start))
	}()

	vacancyData, err := s.vacancyProvider.DataProfession(ctx, profession.VacancyQuery, area)
	if err != nil {
		log.Error("vacancy.fetch_failed", slogx.Err(err))
		return fmt.Errorf("%s: fetch vacancy data: %w", op, err)
	}

	log.Debug("vacancy.fetched", "vacancy_count", len(vacancyData))

	formalSkills := s.aggregateFormalSkills(vacancyData)
	extractedSkills := s.extractSkillsFromText(ctx, vacancyData, formalSkills)

	if saveToDB {
		if err := s.statProvider.SaveStat(ctx, sessionID, profession.ID, len(vacancyData)); err != nil {
			log.Warn("stat.save_failed", slogx.Err(err))
		} else {
			log.Debug("stat.saved")
		}

		if err := s.skillsProvider.SaveFormalSkills(ctx, sessionID, profession.ID, formalSkills); err != nil {
			log.Warn("formal_skills.save_failed", slogx.Err(err))
		} else {
			log.Debug("formal_skills.saved", "skill_count", len(formalSkills))
		}

		if err := s.skillsProvider.SaveExtractedSkills(ctx, sessionID, profession.ID, extractedSkills); err != nil {
			log.Warn("extracted_skills.save_failed", slogx.Err(err))
		} else {
			log.Debug("extracted_skills.saved", "skill_count", len(extractedSkills))
		}
	}

	if s.cache != nil {
		if err := s.saveToCache(ctx, profession, vacancyData, formalSkills, extractedSkills); err != nil {
			log.Warn("cache.save_failed", slogx.Err(err))
		} else {
			log.Debug("cache.saved")
		}
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

func (s *Scraper) extractSkillsFromText(ctx context.Context, data []entity.VacancyData, whiteList map[string]int) map[string]int {
	log := loggerctx.FromContext(ctx)

	result := make(map[string]int)
	for _, d := range data {
		extracted, err := s.extractor.ExtractSkills(d.Description, whiteList, ngram)
		if err != nil {
			log.Warn("extract_failed", slogx.Err(err), "description_preview", truncate(d.Description, 100))
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

func (s *Scraper) saveToCache(
	ctx context.Context,
	profession entity.Profession,
	vacancyData []entity.VacancyData,
	formalSkills map[string]int,
	extractedSkills map[string]int,
) error {
	if s.cache == nil {
		return nil
	}

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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
