package v1

import (
	"log/slog"
	"net/http"
	"psa/internal/controller/http/v1/handler"
	"psa/internal/controller/http/v1/handler/admin"
	"psa/internal/controller/http/v1/handler/public"
)

type Router struct {
	log                    *slog.Logger
	authHandler            *public.AuthHandler
	professionAdminHandler *admin.ProfessionAdminHandler
	professionHandler      *public.ProfessionHandler
}

func New(
	log *slog.Logger,
	authHandler *public.AuthHandler,
	professionAdminHandler *admin.ProfessionAdminHandler,
	professionHandler *public.ProfessionHandler,
) *Router {
	return &Router{
		log:                    log,
		authHandler:            authHandler,
		professionAdminHandler: professionAdminHandler,
		professionHandler:      professionHandler,
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

	// Ping pong
	mux.HandleFunc("GET /ping", r.ping)
}

func (r *Router) RegisterAdminRoutes(mux *http.ServeMux) {
	// Profession admin routes
	mux.HandleFunc("GET /professions", handler.Handle(r.professionAdminHandler.ListAllProfessions))
	mux.HandleFunc("POST /professions", handler.Handle(r.professionAdminHandler.Create))
	mux.HandleFunc("PUT /professions/{id}", handler.Handle(r.professionAdminHandler.Change))
}

func (r *Router) ping(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}
