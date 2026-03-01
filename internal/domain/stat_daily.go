package domain

import (
	"time"

	"github.com/google/uuid"
)

type StatDailyPoint struct {
	Date         time.Time `json:"date"`
	VacancyCount int32     `json:"vacancy_count"`
}

type ProfessionTrend struct {
	ProfessionID   uuid.UUID        `json:"profession_id"`
	ProfessionName string           `json:"profession_name"`
	Data           []StatDailyPoint `json:"data"`
}
