package middleware

import (
	"log/slog"
	"net/http"

	"psa/internal/config"
	"psa/internal/handler/http/middleware/auth"
	"psa/internal/handler/http/middleware/cors"
	"psa/internal/handler/http/middleware/logger"
)

type Manager struct {
	log            *slog.Logger
	tokenValidator auth.TokenValidator
	corsConfig     config.CORS
	corsMiddleware *cors.Middleware
}

func NewManager(log *slog.Logger, tokenValidator auth.TokenValidator, corsConfig config.CORS) (*Manager, error) {
	corsMw, err := cors.NewMiddleware(corsConfig)
	if err != nil {
		return nil, err
	}

	return &Manager{
		log:            log,
		tokenValidator: tokenValidator,
		corsConfig:     corsConfig,
		corsMiddleware: corsMw,
	}, nil
}

func (m *Manager) CORS() Middleware {
	return m.corsMiddleware.Handler()
}

func (m *Manager) Logger() func(http.Handler) http.Handler {
	return logger.NewLoggerMiddleware(m.log)
}

func (m *Manager) Auth() *auth.Middleware {
	return auth.NewAuthMiddleware(m.tokenValidator)
}

func (m *Manager) DefaultChain() *Chain {
	return NewChain().Add(
		m.CORS(),
		m.Logger(),
	)
}

func (m *Manager) AuthChain() *Chain {
	authMw := m.Auth()

	return NewChain().Add(
		m.CORS(),
		m.Logger(),
		authMw.Authenticate,
	)
}

func (m *Manager) AdminChain() *Chain {
	authMw := m.Auth()

	return NewChain().Add(
		m.CORS(),
		m.Logger(),
		authMw.Authenticate,
		authMw.RequireRole("admin"),
	)
}
