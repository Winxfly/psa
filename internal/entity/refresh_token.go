package entity

import (
	"github.com/google/uuid"
	"time"
)

type RefreshToken struct {
	UserID      uuid.UUID `json:"user_id"`
	HashedToken string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}
