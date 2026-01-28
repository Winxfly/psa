package http

import (
	"log/slog"
	"net/http"
	"psa/internal/controller/http/middleware"
	"psa/internal/controller/http/middleware/auth"
	v1 "psa/internal/controller/http/v1"
	"psa/internal/controller/http/v1/handler/admin"
	"psa/internal/controller/http/v1/handler/public"
	"strings"
)

type V1Handlers struct {
	AuthPublic       *public.AuthHandler
	ProfessionPublic *public.ProfessionHandler
	ProfessionAdmin  *admin.ProfessionAdminHandler
}

// NewRouter creates a root router, installs middleware, and connects API versions.
func NewRouter(log *slog.Logger, handlers V1Handlers, tokenValidator auth.TokenValidator) http.Handler {
	mw := middleware.NewManager(log, tokenValidator)

	// v1 router
	v1Router := v1.New(log, handlers.AuthPublic, handlers.ProfessionAdmin, handlers.ProfessionPublic)

	// mux
	root := http.NewServeMux()
	publicMux := http.NewServeMux()
	adminMux := http.NewServeMux()

	// register routes
	v1Router.RegisterPublicRoutes(publicMux)
	v1Router.RegisterAdminRoutes(adminMux)

	// protect admin
	authMw := mw.Auth()

	adminHandler := authMw.Authenticate(
		authMw.RequireAdmin(adminMux),
	)

	// version routing
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/admin")
			adminHandler.ServeHTTP(w, r)
			return
		}
		publicMux.ServeHTTP(w, r)
	})))

	// health check
	root.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mw.Logger()(root)
}
