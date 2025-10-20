package entity

import "github.com/google/uuid"

type Skill struct {
	ID           uuid.UUID `json:"id"`
	ProfessionID uuid.UUID `json:"profession_id"`
	Skill        string    `json:"skill"`
	Count        int32     `json:"count"`
	ScrapedAtID  uuid.UUID `json:"scraped_at_id"`
}

type SkillResponse struct {
	Skill string `json:"skill"`
	Count int32  `json:"count"`
}
