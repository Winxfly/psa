package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"psa/internal/config"
	"psa/internal/handler/http/middleware"
	"psa/internal/handler/http/middleware/auth"
	v1 "psa/internal/handler/http/v1"
	"psa/internal/handler/http/v1/handler/admin"
	"psa/internal/handler/http/v1/handler/public"
)

type V1Handlers struct {
	AuthPublic       *public.AuthHandler
	ProfessionPublic *public.ProfessionHandler
	ProfessionAdmin  *admin.ProfessionAdminHandler
	Trend            *public.TrendHandler
}

// NewRouter creates a root router, installs middleware, and connects API versions.
func NewRouter(log *slog.Logger, handlers V1Handlers, tokenValidator auth.TokenValidator, corsConfig config.CORS) (http.Handler, error) {
	if handlers.AuthPublic == nil {
		return nil, fmt.Errorf("NewRouter: nil AuthPublic handler")
	}
	if handlers.ProfessionPublic == nil {
		return nil, fmt.Errorf("NewRouter: nil ProfessionPublic handler")
	}
	if handlers.ProfessionAdmin == nil {
		return nil, fmt.Errorf("NewRouter: nil ProfessionAdmin handler")
	}
	if handlers.Trend == nil {
		return nil, fmt.Errorf("NewRouter: nil Trend handler")
	}

	mw, err := middleware.NewManager(log, tokenValidator, corsConfig)
	if err != nil {
		return nil, err
	}

	// v1 router
	v1Router := v1.New(handlers.AuthPublic, handlers.ProfessionAdmin, handlers.ProfessionPublic, handlers.Trend)

	// mux
	root := http.NewServeMux()
	publicMux := http.NewServeMux()
	adminMux := http.NewServeMux()

	// register routes
	v1Router.RegisterPublicRoutes(publicMux)
	v1Router.RegisterAdminRoutes(adminMux)

	// apply middleware chain
	publicHandler := mw.DefaultChain().Then(publicMux)
	adminHandler := mw.AdminChain().Then(adminMux)

	// version routing
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/admin")
			adminHandler.ServeHTTP(w, r)
			return
		}
		publicHandler.ServeHTTP(w, r)
	})))

	// health check
	healthHandler := mw.DefaultChain().ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	root.Handle("GET /health", healthHandler)

	return root, nil
}
