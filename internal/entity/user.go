package entity

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID `json:"id"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"-"`
	IsAdmin        bool      `json:"is_admin"`
	CreatedAt      time.Time `json:"created_at"`
}
