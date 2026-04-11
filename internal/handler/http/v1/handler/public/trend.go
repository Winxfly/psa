package public

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

type TrendProvider interface {
	ProfessionTrend(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionTrend, error)
}

type TrendHandler struct {
	provider TrendProvider
}

func NewTrendHandler(provider TrendProvider) *TrendHandler {
	return &TrendHandler{
		provider: provider,
	}
}

type trendPoint struct {
	Date         string `json:"date"`
	VacancyCount int32  `json:"vacancy_count"`
}

type professionTrendResponse struct {
	ProfessionID   string       `json:"profession_id"`
	ProfessionName string       `json:"profession_name"`
	Data           []trendPoint `json:"data"`
}

func (h *TrendHandler) GetProfessionTrend(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professionID, err := handler.PathUUID(r, "id")
	if err != nil {
		log.Warn("trend_invalid_id", slogx.Err(err))
		return handler.StatusBadRequest("Invalid profession ID")
	}

	trend, err := h.provider.ProfessionTrend(ctx, professionID)
	if err != nil {
		if errors.Is(err, domain.ErrProfessionNotFound) {
			return handler.StatusNotFound("Profession trend not found")
		}
		log.Error("trend_failed", "profession_id", professionID, slogx.Err(err))
		return handler.StatusInternalServerError("Failed to get profession trend")
	}

	resp := professionTrendResponse{
		ProfessionID:   trend.ProfessionID.String(),
		ProfessionName: trend.ProfessionName,
		Data:           make([]trendPoint, len(trend.Data)),
	}

	for i, point := range trend.Data {
		resp.Data[i] = trendPoint{
			Date:         point.Date.Format(time.RFC3339),
			VacancyCount: point.VacancyCount,
		}
	}

	log.Debug("trend_success", "profession_id", professionID, "points_count", len(trend.Data))

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}
