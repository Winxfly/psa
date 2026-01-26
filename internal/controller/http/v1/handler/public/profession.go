package public

import (
	"context"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"psa/internal/controller/http/v1/handler"
	"psa/internal/controller/http/v1/response"
	"psa/internal/entity"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

// TODO: поправить dto (типы данных), подумать надо логированием

type ProfessionProvider interface {
	ActiveProfessions(ctx context.Context) ([]entity.ActiveProfession, error)
	ProfessionSkills(ctx context.Context, professionID uuid.UUID) (*entity.ProfessionDetail, error)
}

type ProfessionHandler struct {
	log      *slog.Logger
	provider ProfessionProvider
}

func NewProfessionHandler(log *slog.Logger, provider ProfessionProvider) *ProfessionHandler {
	return &ProfessionHandler{
		log:      log,
		provider: provider,
	}
}

func (h *ProfessionHandler) ListProfessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	professions, err := h.provider.ActiveProfessions(ctx)
	if err != nil {
		loggerctx.FromContext(ctx).Error("Failed to get professions", slogx.Err(err))
		return handler.StatusInternalServerError("Failed to get professions")
	}

	resp := make([]response.ProfessionResponse, len(professions))
	for i, p := range professions {
		resp[i] = response.ProfessionResponse{
			ID:           p.ID.String(),
			Name:         p.Name,
			VacancyQuery: p.VacancyQuery,
		}
	}

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ProfessionHandler) LastProfessionDetails(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	professionID, err := handler.PathUUID(r, "id")
	if err != nil {
		http.Error(w, "Invalid profession ID", http.StatusBadRequest)
		return handler.StatusBadRequest("Invalid profession ID")
	}

	profession, err := h.provider.ProfessionSkills(ctx, professionID)
	if err != nil {
		loggerctx.FromContext(ctx).Error("Profession not found", slogx.Err(err))
		return handler.StatusNotFound("Profession not found")
	}

	// TODO: подумать над этой простыней бесполезности (dto ради чего, если структура такая же как и в entity?)

	resp := response.ProfessionDetailResponse{
		ProfessionID:    profession.ProfessionID.String(),
		ProfessionName:  profession.ProfessionName,
		ScrapedAt:       profession.ScrapedAt,
		VacancyCount:    profession.VacancyCount,
		FormalSkills:    make([]response.SkillResponse, len(profession.FormalSkills)),
		ExtractedSkills: make([]response.SkillResponse, len(profession.ExtractedSkills)),
	}

	for i, skill := range profession.FormalSkills {
		resp.FormalSkills[i] = response.SkillResponse{
			Skill: skill.Skill,
			Count: skill.Count,
		}
	}

	for i, skill := range profession.ExtractedSkills {
		resp.ExtractedSkills[i] = response.SkillResponse{
			Skill: skill.Skill,
			Count: skill.Count,
		}
	}

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}
