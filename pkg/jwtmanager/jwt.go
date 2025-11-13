package jwtmanager

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
)

var (
	ErrInvalidUserID = errors.New("invalid user id")
)

type Manager struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	issuer          string
}

type TokenClaims struct {
	TokenType string `json:"token_type"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func NewJWT(secret string, accessTokenTTL, refreshTokenTTL time.Duration, issuer string) *Manager {
	return &Manager{
		secret:          []byte(secret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		issuer:          issuer,
	}
}

func (j *Manager) GenerateAccessToken(userID uuid.UUID, role string) (string, error) {
	claims := TokenClaims{
		TokenType: "access_token",
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    j.issuer,
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(j.secret)
}

func (j *Manager) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	claims := TokenClaims{
		TokenType: "refresh_token",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    j.issuer,
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(j.secret)
}

func (j *Manager) ParseToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	hashedBytes := h.Sum(nil)

	return base64.StdEncoding.EncodeToString(hashedBytes)
}

func (j *Manager) ExtractUserID(token string) (uuid.UUID, error) {
	claims, err := j.ParseToken(token)
	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %v", ErrInvalidUserID, err)
	}

	return userID, nil
}

func (c *TokenClaims) IsAccessToken() bool {
	return c.TokenType == "access_token"
}

func (c *TokenClaims) IsRefreshToken() bool {
	return c.TokenType == "refresh_token"
}
