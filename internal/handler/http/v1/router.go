package v1

import (
	"net/http"

	"psa/internal/handler/http/v1/handler"
	"psa/internal/handler/http/v1/handler/admin"
	"psa/internal/handler/http/v1/handler/public"
)

type Router struct {
	authHandler            *public.AuthHandler
	professionAdminHandler *admin.ProfessionAdminHandler
	professionHandler      *public.ProfessionHandler
	trendHandler           *public.TrendHandler
}

func New(
	authHandler *public.AuthHandler,
	professionAdminHandler *admin.ProfessionAdminHandler,
	professionHandler *public.ProfessionHandler,
	trendHandler *public.TrendHandler,
) *Router {
	return &Router{
		authHandler:            authHandler,
		professionAdminHandler: professionAdminHandler,
		professionHandler:      professionHandler,
		trendHandler:           trendHandler,
	}
}

func (r *Router) RegisterPublicRoutes(mux *http.ServeMux) {
	// Auth routes
	mux.HandleFunc("POST /auth/signin", handler.Handle(r.authHandler.Signin))
	mux.HandleFunc("POST /auth/refresh", handler.Handle(r.authHandler.Refresh))
	mux.HandleFunc("POST /auth/logout", handler.Handle(r.authHandler.Logout))

	// Profession routes
	mux.HandleFunc("GET /professions", handler.Handle(r.professionHandler.ListProfessions))
	mux.HandleFunc("GET /professions/{id}/latest", handler.Handle(r.professionHandler.LastProfessionDetails))
	mux.HandleFunc("GET /professions/{id}/trend", handler.Handle(r.trendHandler.GetProfessionTrend))

	// Health check
	mux.HandleFunc("GET /health", r.health)
}

func (r *Router) RegisterAdminRoutes(mux *http.ServeMux) {
	// Profession admin routes
	mux.HandleFunc("GET /professions", handler.Handle(r.professionAdminHandler.ListAllProfessions))
	mux.HandleFunc("POST /professions", handler.Handle(r.professionAdminHandler.Create))
	mux.HandleFunc("PUT /professions/{id}", handler.Handle(r.professionAdminHandler.Change))

	// Scraping admin routes
	mux.HandleFunc("POST /scraping/trigger", handler.Handle(r.professionAdminHandler.TriggerScraping))
}

func (r *Router) health(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
