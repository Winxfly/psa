package entity

import (
	"github.com/google/uuid"
	"time"
)

type Scraping struct {
	ID        uuid.UUID `json:"id"`
	ScrapedAt time.Time `json:"scraped_at"`
}
