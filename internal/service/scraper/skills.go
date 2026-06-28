package scraper

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

const (
	ngram = 3
)

func (s *Scraper) aggregateFormalSkills(data []domain.VacancyData) map[string]int {
	skills := make(map[string]int)
	for _, d := range data {
		for _, skill := range d.Skills {
			skills[skill]++
		}
	}
	return skills
}

func filterSkillsByMinCount(skills map[string]int, minCount int) map[string]int {
	result := make(map[string]int)
	for skill, count := range skills {
		if count >= minCount {
			result[skill] = count
		}
	}
	return result
}

func filterSkillsByBlacklist(skills map[string]int, blacklist map[string]struct{}) map[string]int {
	result := make(map[string]int)
	for skill, count := range skills {
		if _, blocked := blacklist[skill]; blocked {
			continue
		}
		result[skill] = count
	}
	return result
}

func (s *Scraper) extractSkillsFromText(ctx context.Context, descriptions []string, whitelist map[string]int) map[string]int {
	log := loggerctx.FromContext(ctx)

	result := make(map[string]int)
	for _, d := range descriptions {
		extracted, err := s.extractor.ExtractSkills(d, whitelist, ngram)
		if err != nil {
			log.Warn("extract_failed", slogx.Err(err), "description_preview", truncate(d, 100))
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
		if result[i].Count == result[j].Count {
			return result[i].Skill < result[j].Skill
		}

		return result[i].Count > result[j].Count
	})

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildGlobalWhitelist(storedCorpus map[uuid.UUID]map[string]int, items []professionScrapeData) map[string]int {
	whitelist := make(map[string]int)

	for _, skills := range storedCorpus {
		for skill, count := range skills {
			whitelist[skill] += count
		}
	}

	for _, item := range items {
		for skill, count := range item.filteredFormalSkills {
			whitelist[skill] += count
		}
	}

	whitelist = filterSkillsByBlacklist(whitelist, skillBlacklist)
	whitelist = filterSkillsByMinCount(whitelist, whitelistSkillMinCount)

	return whitelist
}

var skillBlacklist = map[string]struct{}{
	"с":               {},
	"разработка":      {},
	"обучение":        {},
	"ответственность": {},
}
