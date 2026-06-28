package scraper

import (
	"context"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
)

func (s *Scraper) saveToCache(ctx context.Context, item *professionScrapeData, scrapedAt time.Time) error {
	if s.cache == nil {
		return nil
	}

	cacheData := &domain.ProfessionDetail{
		ProfessionID:    item.profession.ID,
		ProfessionName:  item.profession.Name,
		ScrapedAt:       scrapedAt.Format(time.RFC3339),
		VacancyCount:    int32(item.totalFound),
		FormalSkills:    s.transformSkillsSort(item.filteredFormalSkills),
		ExtractedSkills: s.transformSkillsSort(item.extractedSkills),
	}

	return s.cache.SaveProfessionData(ctx, cacheData)
}

func (s *Scraper) saveArchiveResult(ctx context.Context, item *professionScrapeData,
	sessionID uuid.UUID, scrapedAt time.Time) error {
	result := makeProfessionScrapeResult(item)

	return s.resultSaver.SaveArchiveProfessionResult(ctx, sessionID, scrapedAt, result)
}

func (s *Scraper) saveDailyResult(ctx context.Context, item *professionScrapeData, scrapedAt time.Time) error {
	result := makeProfessionScrapeResult(item)

	return s.resultSaver.SaveDailyProfessionResult(ctx, scrapedAt, result)
}

func makeProfessionScrapeResult(item *professionScrapeData) domain.ProfessionScrapeResult {
	return domain.ProfessionScrapeResult{
		ProfessionID:    item.profession.ID,
		VacancyCount:    item.totalFound,
		FormalSkills:    item.filteredFormalSkills,
		ExtractedSkills: item.extractedSkills,
	}
}
