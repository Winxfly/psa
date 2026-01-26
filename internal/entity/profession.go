package entity

import (
	"errors"
	"github.com/google/uuid"
)

var (
	ErrProfessionNotFound      = errors.New("profession not found")
	ErrProfessionAlreadyExists = errors.New("profession already exists")
	ErrInvalidProfessionName   = errors.New("invalid profession name")
	ErrInvalidProfessionQuery  = errors.New("invalid profession query")
	ErrInvalidProfessionID     = errors.New("invalid profession id")
)

type Profession struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VacancyQuery string    `json:"vacancy_query"`
	IsActive     bool      `json:"is_active"`
}

type ActiveProfession struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VacancyQuery string    `json:"vacancy_query"`
}

type ProfessionDetail struct {
	ProfessionID    uuid.UUID       `json:"profession_id"`
	ProfessionName  string          `json:"profession_name"`
	ScrapedAt       string          `json:"scraped_at"`
	VacancyCount    int32           `json:"vacancy_count"`
	FormalSkills    []SkillResponse `json:"formal_skills"`
	ExtractedSkills []SkillResponse `json:"extracted_skills"`
}
