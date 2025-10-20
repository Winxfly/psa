package v1

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"psa/internal/usecase/provider"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

type professionHandler struct {
	provider *provider.Provider
}

func NewProfessionHandler(provider *provider.Provider) *professionHandler {
	return &professionHandler{
		provider: provider,
	}
}

func (h *professionHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/professions", h.listProfessions)
	mux.HandleFunc("GET /v1/profession/{id}/skills", h.professionSkills)
}

func (h *professionHandler) listProfessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	professions, err := h.provider.ActiveProfessions(ctx)
	if err != nil {
		loggerctx.FromContext(ctx).Error("Failed to get professions", slogx.Err(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(professions)
}

func (h *professionHandler) professionSkills(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	professionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid profession ID", http.StatusBadRequest)
		return
	}

	profession, err := h.provider.ProfessionSkills(ctx, professionID)
	if err != nil {
		loggerctx.FromContext(ctx).Error("Failed to get profession", slogx.Err(err))
		http.Error(w, "Profession not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profession)
}
