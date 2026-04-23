package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"psa/internal/config"
	"psa/internal/handler/http/middleware/auth"
	"psa/internal/handler/http/middleware/cors"
	"psa/internal/handler/http/middleware/logger"
	httpmetrics "psa/internal/handler/http/middleware/metrics"
	appmetrics "psa/internal/metrics"
)

type Manager struct {
	log            *slog.Logger
	tokenValidator auth.TokenValidator
	corsConfig     config.CORS
	corsMiddleware *cors.Middleware
	httpMetrics    *appmetrics.HTTPMetrics
}

func NewManager(log *slog.Logger, tokenValidator auth.TokenValidator, corsConfig config.CORS, httpMetrics *appmetrics.HTTPMetrics) (*Manager, error) {
	corsMw, err := cors.NewMiddleware(corsConfig)
	if err != nil {
		return nil, err
	}
	if httpMetrics == nil {
		return nil, fmt.Errorf("NewManager: nil HTTP metrics")
	}

	return &Manager{
		log:            log,
		tokenValidator: tokenValidator,
		corsConfig:     corsConfig,
		corsMiddleware: corsMw,
		httpMetrics:    httpMetrics,
	}, nil
}

func (m *Manager) CORS() Middleware {
	return m.corsMiddleware.Handler()
}

func (m *Manager) Logger() func(http.Handler) http.Handler {
	return logger.NewLoggerMiddleware(m.log)
}

func (m *Manager) Metrics(scope string) Middleware {
	return httpmetrics.New(scope, m.httpMetrics).Handler()
}

func (m *Manager) Auth() *auth.Middleware {
	return auth.NewAuthMiddleware(m.tokenValidator)
}

func (m *Manager) DefaultChain() *Chain {
	return NewChain().Add(
		m.Metrics("public"),
		m.CORS(),
		m.Logger(),
	)
}

func (m *Manager) SystemChain() *Chain {
	return NewChain().Add(
		m.Metrics("system"),
		m.CORS(),
		m.Logger(),
	)
}

func (m *Manager) AuthChain() *Chain {
	authMw := m.Auth()

	return NewChain().Add(
		m.Metrics("auth"),
		m.CORS(),
		m.Logger(),
		authMw.Authenticate,
	)
}

func (m *Manager) AdminChain() *Chain {
	authMw := m.Auth()

	return NewChain().Add(
		m.Metrics("admin"),
		m.CORS(),
		m.Logger(),
		authMw.Authenticate,
		authMw.RequireRole("admin"),
	)
}
