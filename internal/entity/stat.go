package entity

import "github.com/google/uuid"

type Stat struct {
	ID           uuid.UUID `json:"id"`
	ProfessionID uuid.UUID `json:"profession_id"`
	VacancyCount int32     `json:"vacancy_count"`
	ScrapedAtID  uuid.UUID `json:"scraped_at_id"`
}
