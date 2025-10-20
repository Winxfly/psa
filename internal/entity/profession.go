package entity

import "github.com/google/uuid"

type Profession struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VacancyQuery string    `json:"vacancy_query"`
	IsActive     bool      `json:"is_active"`
}
