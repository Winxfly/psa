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
		log.Warn("trend_not_found", "profession_id", professionID, slogx.Err(err))
		return handler.StatusNotFound("Profession trend not found")
	}

	resp := response.ProfessionTrendResponse{
		ProfessionID:   trend.ProfessionID.String(),
		ProfessionName: trend.ProfessionName,
		Data:           make([]response.TrendPoint, len(trend.Data)),
	}

	for i, point := range trend.Data {
		resp.Data[i] = response.TrendPoint{
			Date:         point.Date.Format(time.RFC3339),
			VacancyCount: point.VacancyCount,
		}
	}

	log.Debug("trend_success", "profession_id", professionID, "points_count", len(trend.Data))

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}
