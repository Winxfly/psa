package http

import (
	"log/slog"
	"net/http"
	"psa/internal/controller/http/middleware/logger"
	v1 "psa/internal/controller/http/v1"
	"psa/internal/usecase/provider"
)

// NewRouter creates a root router, installs middleware, and connects API versions.
func NewRouter(log *slog.Logger, professionProvider *provider.Provider) http.Handler {
	mux := http.NewServeMux()

	// connect v1 API
	v1Router := v1.New(professionProvider)
	v1Router.Register(mux)

	// mount v1 под /v1/*
	//mux.Handle("/v1/", http.StripPrefix("/v1", v1Mux))
	handler := logger.New(log)(mux)

	return handler
}
