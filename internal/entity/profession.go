package entity

import "github.com/google/uuid"

type Profession struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VacancyQuery string    `json:"vacancy_query"`
	IsActive     bool      `json:"is_active"`
}

type ProfessionDetail struct {
	ProfessionID    uuid.UUID       `json:"profession_id"`
	ProfessionName  string          `json:"profession_name"`
	ScrapedAt       string          `json:"scraped_at"`
	VacancyCount    int32           `json:"vacancy_count"`
	FormalSkills    []SkillResponse `json:"formal_skills"`
	ExtractedSkills []SkillResponse `json:"extracted_skills"`
}
