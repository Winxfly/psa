package v1

import (
	"net/http"
	"psa/internal/usecase/provider"
)

type router struct {
	professionHandler *professionHandler
}

func New(professionProvider *provider.Provider) *router {
	return &router{
		professionHandler: NewProfessionHandler(professionProvider),
	}
}

func (r *router) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/ping", r.ping)

	// Public
	r.professionHandler.Register(mux)
}

func (r *router) ping(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}
