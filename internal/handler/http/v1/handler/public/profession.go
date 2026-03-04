package public

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/internal/handler/http/v1/response"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

type ProfessionProvider interface {
	ActiveProfessions(ctx context.Context) ([]domain.ActiveProfession, error)
	ProfessionSkills(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionDetail, error)
	ProfessionTrend(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionTrend, error)
}

type ProfessionHandler struct {
	provider ProfessionProvider
}

func NewProfessionHandler(provider ProfessionProvider) *ProfessionHandler {
	return &ProfessionHandler{
		provider: provider,
	}
}

func (h *ProfessionHandler) ListProfessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professions, err := h.provider.ActiveProfessions(ctx)
	if err != nil {
		log.Error("profession_list_failed", slogx.Err(err))
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

	log.Debug("profession_list_success", "count", len(professions))

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ProfessionHandler) LastProfessionDetails(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professionID, err := handler.PathUUID(r, "id")
	if err != nil {
		log.Warn("profession_details_invalid_id", slogx.Err(err))
		return handler.StatusBadRequest("Invalid profession ID")
	}

	includeTrend := r.URL.Query().Get("trend") == "true"

	profession, err := h.provider.ProfessionSkills(ctx, professionID)
	if err != nil {
		log.Warn("profession_details_not_found", "profession_id", professionID)
		return handler.StatusNotFound("Profession not found")
	}

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

	if includeTrend {
		trend, err := h.provider.ProfessionTrend(ctx, professionID)
		if err != nil {
			log.Warn("profession_trend_failed", "profession_id", professionID, slogx.Err(err))
		} else {
			resp.Trend = make([]response.TrendPoint, len(trend.Data))
			for i, point := range trend.Data {
				resp.Trend[i] = response.TrendPoint{
					Date:         point.Date.Format(time.RFC3339),
					VacancyCount: point.VacancyCount,
				}
			}
			log.Debug("profession_trend_loaded", "profession_id", professionID, "points_count", len(trend.Data))
		}
	}

	log.Debug("profession_details_success", "profession_id", professionID)

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}
