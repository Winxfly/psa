package public

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
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

type professionResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
}

func (h *ProfessionHandler) ListProfessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professions, err := h.provider.ActiveProfessions(ctx)
	if err != nil {
		log.Error("profession_list_failed", slogx.Err(err))
		return handler.StatusInternalServerError("Failed to get professions")
	}

	resp := make([]professionResponse, len(professions))
	for i, p := range professions {
		resp[i] = professionResponse{
			ID:           p.ID.String(),
			Name:         p.Name,
			VacancyQuery: p.VacancyQuery,
		}
	}

	log.Debug("profession_list_success", "count", len(professions))

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

type skillResponse struct {
	Skill string `json:"skill"`
	Count int32  `json:"count"`
}

type trendProfession struct {
	Date         string `json:"date"`
	VacancyCount int32  `json:"vacancy_count"`
}

type professionDetailResponse struct {
	ProfessionID    string            `json:"profession_id"`
	ProfessionName  string            `json:"profession_name"`
	ScrapedAt       string            `json:"scraped_at"`
	VacancyCount    int32             `json:"vacancy_count"`
	FormalSkills    []skillResponse   `json:"formal_skills"`
	ExtractedSkills []skillResponse   `json:"extracted_skills"`
	Trend           []trendProfession `json:"trend,omitempty"`
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

	resp := professionDetailResponse{
		ProfessionID:    profession.ProfessionID.String(),
		ProfessionName:  profession.ProfessionName,
		ScrapedAt:       profession.ScrapedAt,
		VacancyCount:    profession.VacancyCount,
		FormalSkills:    make([]skillResponse, len(profession.FormalSkills)),
		ExtractedSkills: make([]skillResponse, len(profession.ExtractedSkills)),
	}

	for i, skill := range profession.FormalSkills {
		resp.FormalSkills[i] = skillResponse{
			Skill: skill.Skill,
			Count: skill.Count,
		}
	}

	for i, skill := range profession.ExtractedSkills {
		resp.ExtractedSkills[i] = skillResponse{
			Skill: skill.Skill,
			Count: skill.Count,
		}
	}

	if includeTrend {
		trend, err := h.provider.ProfessionTrend(ctx, professionID)
		if err != nil {
			log.Warn("profession_trend_failed", "profession_id", professionID, slogx.Err(err))
		} else {
			resp.Trend = make([]trendProfession, len(trend.Data))
			for i, point := range trend.Data {
				resp.Trend[i] = trendProfession{
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
