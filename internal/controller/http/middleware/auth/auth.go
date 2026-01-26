package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"psa/internal/entity"
	"psa/pkg/logger/loggerctx"
	"strings"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*entity.TokenClaims, error)
}

type Middleware struct {
	tokenValidator TokenValidator
}

func NewAuthMiddleware(tokenValidator TokenValidator) *Middleware {
	return &Middleware{
		tokenValidator: tokenValidator,
	}
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := loggerctx.FromContext(ctx)

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Warn("Authorization header missing")
			m.respondWithError(w, http.StatusUnauthorized, "Authorization header required")

			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			log.Warn("Invalid authorization header format")
			m.respondWithError(w, http.StatusUnauthorized, "Authorization header format must be Bearer {token}")

			return
		}

		token := parts[1]
		claims, err := m.tokenValidator.ValidateToken(ctx, token)
		if err != nil {
			log.Warn("Token validation failed", "error", err)
			m.respondWithError(w, http.StatusUnauthorized, "Invalid token")

			return
		}

		log.Debug("Token validated", "user_id", claims.UserID, "role", claims.Role)

		ctx = context.WithValue(ctx, userContextKey, claims)

		ctx = loggerctx.WithLogger(ctx, log.With("user_id", claims.UserID))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := loggerctx.FromContext(r.Context())

			claims, ok := r.Context().Value(userContextKey).(*entity.TokenClaims)
			if !ok {
				log.Warn("Claims no found in context")
				m.respondWithError(w, http.StatusUnauthorized, "Authentication required")

				return
			}

			if claims.Role != requiredRole {
				log.Warn("Insufficient permissions", "user_role", claims.Role, "required_role", requiredRole)
				m.respondWithError(w, http.StatusForbidden, "Insufficient permissions")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireRole("admin")(next)
}

func (m *Middleware) respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func GetUserFromContext(ctx context.Context) (*entity.TokenClaims, error) {
	claims, ok := ctx.Value(userContextKey).(*entity.TokenClaims)
	if !ok {
		return nil, errors.New("user not found in context")
	}

	return claims, nil
}
