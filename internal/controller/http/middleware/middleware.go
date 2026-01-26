package middleware

import (
	"log/slog"
	"net/http"
	"psa/internal/controller/http/middleware/auth"
	"psa/internal/controller/http/middleware/logger"
)

type Manager struct {
	log            *slog.Logger
	tokenValidator auth.TokenValidator
}

func NewManager(log *slog.Logger, tokenValidator auth.TokenValidator) *Manager {
	return &Manager{
		log:            log,
		tokenValidator: tokenValidator,
	}
}

func (m *Manager) Logger() func(http.Handler) http.Handler {
	return logger.NewLoggerMiddleware(m.log)
}

func (m *Manager) Auth() *auth.Middleware {
	return auth.NewAuthMiddleware(m.tokenValidator)
}
