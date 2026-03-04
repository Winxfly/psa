package scraper

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

const (
	ngram = 3
	area  = "113" // Russia
)

type ProfessionProvider interface {
	GetActiveProfessions(ctx context.Context) ([]domain.Profession, error)
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

type DailyStatProvider interface {
	SaveStatDaily(ctx context.Context, professionID uuid.UUID, vacancyCount int, scrapedAt time.Time) error
}

type SupplierPort interface {
	FetchDataProfession(ctx context.Context, query, area string) ([]domain.VacancyData, int, error)
}

type Extractor interface {
	ExtractSkills(text string, whiteList map[string]int, maxNgram int) (map[string]int, error)
}

type CacheProvider interface {
	SaveProfessionData(ctx context.Context, data *domain.ProfessionDetail) error
}

type Scraper struct {
	professionProvider ProfessionProvider
	sessionProvider    SessionProvider
	skillsProvider     SkillsProvider
	statProvider       StatProvider
	dailyStatProvider  DailyStatProvider
	supplierPort       SupplierPort
	extractor          Extractor
	cache              CacheProvider
}

func New(
	professionProvider ProfessionProvider,
	sessionCreator SessionProvider,
	skillSaver SkillsProvider,
	statSaver StatProvider,
	dailyStatSaver DailyStatProvider,
	vacancyFetcher SupplierPort,
	extractor Extractor,
	cache CacheProvider,
) *Scraper {
	return &Scraper{
		professionProvider: professionProvider,
		sessionProvider:    sessionCreator,
		skillsProvider:     skillSaver,
		statProvider:       statSaver,
		dailyStatProvider:  dailyStatSaver,
		supplierPort:       vacancyFetcher,
		extractor:          extractor,
		cache:              cache,
	}
}

func (s *Scraper) ProcessActiveProfessions(ctx context.Context, saveToDB bool) error {
	const op = "service.scraper.ProcessActiveProfessions"
	log := loggerctx.FromContext(ctx).With("op", op)

	start := time.Now()

	var (
		professionsProcessed int
		professionSuccess    int
		professionFailed     int
		totalVacancies       int
	)

	defer func() {
		log.Info("scraping_completed",
			"duration", time.Since(start),
			"profession_processed", professionsProcessed,
			"profession_success", professionSuccess,
			"profession_failed", professionFailed,
			"vacancies_total", totalVacancies)
	}()

	log.Info("scraping_started", "save_to_db", saveToDB)

	professions, err := s.professionProvider.GetActiveProfessions(ctx)
	if err != nil {
		log.Error("get_active_professions_failed", slogx.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("active_professions_loaded", "count", len(professions))

	var sessionID uuid.UUID
	if saveToDB {
		sessionID, err = s.sessionProvider.CreateScrapingSession(ctx)
		if err != nil {
			log.Error("session_create_failed", slogx.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
		log.Info("session_created", "session_id", sessionID)
	} else {
		sessionID = uuid.New()
		log.Info("session_temporary", "session_id", sessionID)
	}

	for _, profession := range professions {
		professionsProcessed++
		totalFound, err := s.processProfession(ctx, profession, sessionID, saveToDB)
		if err != nil {
			log.Error("profession_process_failed", "profession_id", profession.ID,
				"profession_name", profession.Name, slogx.Err(err))
			professionFailed++
			continue
		}
		professionSuccess++
		totalVacancies += totalFound
	}

	return nil
}

func (s *Scraper) processProfession(ctx context.Context, profession domain.Profession, sessionID uuid.UUID, saveToDB bool) (int, error) {
	const op = "service.scraper.processProfession"
	log := loggerctx.FromContext(ctx).With(
		"op", op,
		"profession_id", profession.ID,
		"profession_name", profession.Name,
		"session_id", sessionID,
	)

	log.Info("profession_started")

	start := time.Now()
	defer func() {
		log.Info("profession_completed", "duration", time.Since(start))
	}()

	vacancyData, totalFound, err := s.supplierPort.FetchDataProfession(ctx, profession.VacancyQuery, area)
	if err != nil {
		log.Error("vacancy_fetch_failed", slogx.Err(err))
		return totalFound, fmt.Errorf("%s: fetch vacancy data: %w", op, err)
	}

	log.Debug("vacancy_fetched", "vacancy_count", len(vacancyData), "total_found", totalFound)

	formalSkills := s.aggregateFormalSkills(vacancyData)
	filteredFormalSkills := s.filterRareSkills(formalSkills, 2)
	extractedSkills := s.extractSkillsFromText(ctx, vacancyData, filteredFormalSkills)

	if err := s.dailyStatProvider.SaveStatDaily(ctx, profession.ID, totalFound, time.Now()); err != nil {
		log.Warn("stat_daily_save_failed", slogx.Err(err))
	} else {
		log.Info("stat_daily_saved", "total_found", totalFound)
	}

	if saveToDB {
		if err := s.statProvider.SaveStat(ctx, sessionID, profession.ID, totalFound); err != nil {
			log.Warn("stat_save_failed", slogx.Err(err))
		} else {
			log.Info("stat_saved")
		}

		if err := s.skillsProvider.SaveFormalSkills(ctx, sessionID, profession.ID, filteredFormalSkills); err != nil {
			log.Warn("formal_skills_save_failed", slogx.Err(err))
		} else {
			log.Debug("formal_skills_saved", "skill_count", len(filteredFormalSkills))
		}

		if err := s.skillsProvider.SaveExtractedSkills(ctx, sessionID, profession.ID, extractedSkills); err != nil {
			log.Warn("extracted_skills_save_failed", slogx.Err(err))
		} else {
			log.Debug("extracted_skills_saved", "skill_count", len(extractedSkills))
		}
	}

	if s.cache != nil {
		if err := s.saveToCache(ctx, profession, totalFound, filteredFormalSkills, extractedSkills); err != nil {
			log.Warn("cache_save_failed", slogx.Err(err))
		} else {
			log.Debug("cache_saved")
		}
	}

	return totalFound, nil
}

func (s *Scraper) aggregateFormalSkills(data []domain.VacancyData) map[string]int {
	skills := make(map[string]int)
	for _, d := range data {
		for _, skill := range d.Skills {
			skills[skill]++
		}
	}
	return skills
}

func (s *Scraper) filterRareSkills(skills map[string]int, minCount int) map[string]int {
	result := make(map[string]int)
	for skill, count := range skills {
		if count >= minCount {
			result[skill] = count
		}
	}
	return result
}

func (s *Scraper) extractSkillsFromText(ctx context.Context, data []domain.VacancyData, whiteList map[string]int) map[string]int {
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

func (s *Scraper) transformSkillsSort(skills map[string]int) []domain.SkillResponse {
	result := make([]domain.SkillResponse, 0, len(skills))
	for skill, count := range skills {
		result = append(result, domain.SkillResponse{
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
	profession domain.Profession,
	totalFound int,
	formalSkills map[string]int,
	extractedSkills map[string]int,
) error {
	if s.cache == nil {
		return nil
	}

	cacheData := &domain.ProfessionDetail{
		ProfessionID:    profession.ID,
		ProfessionName:  profession.Name,
		ScrapedAt:       time.Now().Format(time.RFC3339),
		VacancyCount:    int32(totalFound),
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
