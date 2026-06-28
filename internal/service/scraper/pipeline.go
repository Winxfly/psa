package scraper

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

const (
	area = "113" // Russia

	formalSkillMinCount    = 2
	whitelistSkillMinCount = 3
	extractedSkillMinCount = 2
)

type professionScrapeData struct {
	profession           domain.Profession
	totalFound           int
	descriptions         []string
	filteredFormalSkills map[string]int
	extractedSkills      map[string]int
}

func (s *Scraper) collectProfData(ctx context.Context, professions []domain.Profession) ([]professionScrapeData, int, int) {
	const op = "service.scraper.pipeline.collectProfData"

	items := make([]professionScrapeData, 0, len(professions))

	var (
		failed         int
		totalVacancies int
	)

	for _, profession := range professions {
		log := loggerctx.FromContext(ctx).With(
			"op", op,
			"profession_id", profession.ID,
			"profession_name", profession.Name,
		)

		start := time.Now()

		log.Debug("profession_fetch_started")

		vacancyData, totalFound, err := s.supplierPort.FetchDataProfession(ctx, profession.VacancyQuery, area)
		if err != nil {
			log.Error("vacancy_fetch_failed", slogx.Err(err))
			failed++
			continue
		}

		formalSkills := s.aggregateFormalSkills(vacancyData)
		filteredFormalSkills := filterSkillsByMinCount(formalSkills, formalSkillMinCount)
		descriptions := collectDescriptions(vacancyData)

		items = append(items, professionScrapeData{
			profession:           profession,
			totalFound:           totalFound,
			descriptions:         descriptions,
			filteredFormalSkills: filteredFormalSkills,
		})

		totalVacancies += totalFound

		log.Info("profession_fetch_completed",
			"duration", time.Since(start),
			"vacancy_count", len(vacancyData),
			"total_found", totalFound,
			"formal_skill_count", len(filteredFormalSkills),
			"description_count", len(descriptions),
		)
	}

	return items, failed, totalVacancies
}

func (s *Scraper) extractProfSkills(ctx context.Context, items []professionScrapeData, whitelist map[string]int) {
	const op = "service.scraper.pipeline.extractProfSkills"

	for i := range items {
		item := &items[i]

		log := loggerctx.FromContext(ctx).With(
			"op", op,
			"profession_id", item.profession.ID,
			"profession_name", item.profession.Name,
		)

		start := time.Now()

		log.Debug("profession_extract_started",
			"description_count", len(item.descriptions),
			"whitelist_skill_count", len(whitelist),
		)

		extractedSkills := s.extractSkillsFromText(ctx, item.descriptions, whitelist)
		item.extractedSkills = filterSkillsByMinCount(extractedSkills, extractedSkillMinCount)

		log.Info("profession_extract_completed",
			"duration", time.Since(start),
			"extracted_skill_count", len(item.extractedSkills),
		)
	}
}

type saveProfResultsSummary struct {
	success int
	failed  int
}

func (s *Scraper) saveProfResults(ctx context.Context, items []professionScrapeData,
	sessionID uuid.UUID, saveToDB bool, scrapedAt time.Time) saveProfResultsSummary {
	const op = "service.scraper.pipeline.saveProfResults"

	var summary saveProfResultsSummary

	for i := range items {
		item := &items[i]

		log := loggerctx.FromContext(ctx).With(
			"op", op,
			"profession_id", item.profession.ID,
			"profession_name", item.profession.Name,
			"session_id", sessionID,
		)

		ctxWithLogger := loggerctx.WithLogger(ctx, log)
		start := time.Now()

		if saveToDB {
			if err := retry(ctxWithLogger, "save_archive_result", func() error {
				return s.saveArchiveResult(ctxWithLogger, item, sessionID, scrapedAt)
			}); err != nil {
				log.Error("archive_result_save_failed", slogx.Err(err))
				summary.failed++
				continue
			}
		} else {
			if err := retry(ctxWithLogger, "save_daily_result", func() error {
				return s.saveDailyResult(ctxWithLogger, item, scrapedAt)
			}); err != nil {
				log.Error("daily_result_save_failed", slogx.Err(err))
				summary.failed++
				continue
			}
		}

		if err := retry(ctxWithLogger, "save_profession_cache", func() error {
			return s.saveToCache(ctxWithLogger, item, scrapedAt)
		}); err != nil {
			log.Warn("cache_save_failed", slogx.Err(err))
			summary.failed++
			continue
		} else {
			log.Debug("cache_saved")
		}

		summary.success++

		log.Info("profession_save_completed",
			"duration", time.Since(start),
			"save_to_db", saveToDB,
			"cache_saved", true,
			"total_found", item.totalFound,
			"formal_skill_count", len(item.filteredFormalSkills),
			"extracted_skill_count", len(item.extractedSkills),
		)
	}

	return summary
}

func (s *Scraper) processActiveProfessions(ctx context.Context, saveToDB bool) error {
	const op = "service.scraper.pipeline.processActiveProfessions"
	log := loggerctx.FromContext(ctx).With("op", op)

	start := time.Now()
	scrapedAt := nowInBusinessTZ()

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

	var professions []domain.Profession

	if err := retry(ctx, "get_active_professions", func() error {
		var err error
		professions, err = s.professionProvider.GetActiveProfessions(ctx)
		return err
	}); err != nil {
		log.Error("get_active_professions_failed", slogx.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("active_professions_loaded", "count", len(professions))

	items, fetchFailed, total := s.collectProfData(ctx, professions)
	professionsProcessed = len(professions)
	professionFailed = fetchFailed
	totalVacancies = total

	if len(items) == 0 {
		log.Warn("no_professions_collected")
		return nil
	}

	var storedCorpus map[uuid.UUID]map[string]int

	if err := retry(ctx, "get_active_skill_corpus", func() error {
		var err error
		storedCorpus, err = s.corpusProvider.GetActiveSkillCorpus(ctx)
		return err
	}); err != nil {
		log.Warn("skill_corpus_load_failed", slogx.Err(err))
		storedCorpus = nil
	}

	globalWhitelist := buildGlobalWhitelist(storedCorpus, items)
	log.Info("global_whitelist_built",
		"stored_profession_count", len(storedCorpus),
		"fresh_profession_count", len(items),
		"skill_count", len(globalWhitelist),
	)

	s.extractProfSkills(ctx, items, globalWhitelist)

	var sessionID uuid.UUID
	if saveToDB {
		if err := retry(ctx, "create_scraping_session", func() error {
			var err error
			sessionID, err = s.sessionProvider.CreateScrapingSession(ctx)
			return err
		}); err != nil {
			log.Error("session_create_failed", slogx.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}

		log.Info("session_created", "session_id", sessionID)
	} else {
		sessionID = uuid.New()
		log.Debug("session_temporary", "session_id", sessionID)
	}

	saveSummary := s.saveProfResults(ctx, items, sessionID, saveToDB, scrapedAt)

	professionSuccess = saveSummary.success
	professionFailed = fetchFailed + saveSummary.failed

	return nil
}

func collectDescriptions(data []domain.VacancyData) []string {
	descriptions := make([]string, 0, len(data))

	for _, vacancy := range data {
		descriptions = append(descriptions, vacancy.Description)
	}

	return descriptions
}
