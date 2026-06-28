package scraper

import (
	"context"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
)

type ProfessionProvider interface {
	GetActiveProfessions(ctx context.Context) ([]domain.Profession, error)
}

type SessionProvider interface {
	CreateScrapingSession(ctx context.Context) (uuid.UUID, error)
}

type SkillCorpusProvider interface {
	GetActiveSkillCorpus(ctx context.Context) (map[uuid.UUID]map[string]int, error)
}

type ScrapeResultSaver interface {
	SaveArchiveProfessionResult(ctx context.Context, sessionID uuid.UUID, scrapedAt time.Time, result domain.ProfessionScrapeResult) error
	SaveDailyProfessionResult(ctx context.Context, scrapedAt time.Time, result domain.ProfessionScrapeResult) error
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
	corpusProvider     SkillCorpusProvider
	resultSaver        ScrapeResultSaver
	supplierPort       SupplierPort
	extractor          Extractor
	cache              CacheProvider
}

func New(
	professionProvider ProfessionProvider,
	sessionCreator SessionProvider,
	corpusProvider SkillCorpusProvider,
	resultSaver ScrapeResultSaver,
	vacancyFetcher SupplierPort,
	extractor Extractor,
	cache CacheProvider,
) *Scraper {
	return &Scraper{
		professionProvider: professionProvider,
		sessionProvider:    sessionCreator,
		corpusProvider:     corpusProvider,
		resultSaver:        resultSaver,
		supplierPort:       vacancyFetcher,
		extractor:          extractor,
		cache:              cache,
	}
}

// ProcessActiveProfessionsArchive — full scraping (all to db)
func (s *Scraper) ProcessActiveProfessionsArchive(ctx context.Context) error {
	return s.processActiveProfessions(ctx, true)
}

// ProcessActiveProfessionsDaily — daily scraping (stat_daily to db, other to cache)
func (s *Scraper) ProcessActiveProfessionsDaily(ctx context.Context) error {
	return s.processActiveProfessions(ctx, false)
}
