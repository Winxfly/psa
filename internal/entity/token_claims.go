package entity

import (
	"github.com/google/uuid"
	"time"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access_token"
	TokenTypeRefresh TokenType = "refresh_token"
)

type TokenClaims struct {
	UserID    uuid.UUID
	Role      string
	TokenType TokenType
	ExpiresAt time.Time
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
