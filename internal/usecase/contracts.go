package usecase

import (
	"context"
	"github.com/google/uuid"
	"psa/internal/entity"
)

type HHAPI interface {
	DataProfession(ctx context.Context, query string, area string) ([]entity.VacancyData, error)
}

type ProfessionRepo interface {
	GetActiveProfessions(ctx context.Context) ([]entity.Profession, error)
	GetAllProfessions(ctx context.Context) ([]entity.Profession, error)
	GetProfessionByID(ctx context.Context, id uuid.UUID) (entity.Profession, error)
	GetProfessionByName(ctx context.Context, name string) (entity.Profession, error)
	AddProfession(ctx context.Context, profession entity.Profession) (uuid.UUID, error)
	UpdateProfession(ctx context.Context, profession entity.Profession) error
}

type ScrapingRepo interface {
	CreateScrapingSession(ctx context.Context) (uuid.UUID, error)
	GetLatestScraping(ctx context.Context) (entity.Scraping, error)
	GetAllScrapingDates(ctx context.Context) ([]entity.Scraping, error)
	ExistsScrapingSessionInCurrMonth(ctx context.Context) (bool, error)
}

type SkillRepo interface {
	SaveFormalSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error
	SaveExtractedSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error
	GetFormalSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error)
	GetExtractedSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error)
}

type StatRepo interface {
	SaveStat(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, vacancyCount int) error
	GetLatestStatByProfessionID(ctx context.Context, professionID uuid.UUID) (entity.Stat, error)
	GetStatsByProfessionsAndDateRange(ctx context.Context, professionIDs []uuid.UUID, startDate, endDate string) ([]entity.Stat, error)
}

type UserRepo interface {
	GetUserByEmail(ctx context.Context, email string) (*entity.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, refreshToken string) (bool, error)
	ValidateRefreshToken(ctx context.Context, userID uuid.UUID, refreshToken string) (bool, error)
	DeleteRefreshToken(ctx context.Context, userID uuid.UUID, refreshToken string) error
}

type Repository interface {
	ProfessionRepo
	ScrapingRepo
	SkillRepo
	StatRepo

	Close()
}

type Scraper interface {
	ProcessActiveProfessions(ctx context.Context) error
}

type Scheduler interface {
	StartMonthlySchedule() error
	Stop()
}
