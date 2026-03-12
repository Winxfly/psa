package domain

import (
	"time"

	"github.com/google/uuid"
)

type Scraping struct {
	ID        uuid.UUID `json:"id"`
	ScrapedAt time.Time `json:"scraped_at"`
}
