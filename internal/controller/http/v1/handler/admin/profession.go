package admin

import (
	"context"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"psa/internal/controller/http/v1/handler"
	"psa/internal/controller/http/v1/request"
	"psa/internal/controller/http/v1/response"
	"psa/internal/entity"
	"psa/pkg/logger/loggerctx"
)

type ProfessionAdminAccesser interface {
	AllProfessions(ctx context.Context) ([]entity.Profession, error)
	CreateProfession(ctx context.Context, profession entity.Profession) (uuid.UUID, error)
	ChangeProfession(ctx context.Context, profession entity.Profession) error
}

type ProfessionAdminHandler struct {
	log        *slog.Logger
	profession ProfessionAdminAccesser
}

func NewProfessionAdminHandler(log *slog.Logger, profession ProfessionAdminAccesser) *ProfessionAdminHandler {
	return &ProfessionAdminHandler{
		log:        log,
		profession: profession,
	}
}

func (h *ProfessionAdminHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	var req request.CreateProfessionRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		log.Warn("Failed to decode create profession request", "error", err)

		return err
	}

	profession := entity.Profession{
		Name:         req.Name,
		VacancyQuery: req.VacancyQuery,
		IsActive:     true,
	}

	id, err := h.profession.CreateProfession(ctx, profession)
	if err != nil {
		log.Error("Failed to create profession", "name", profession.Name, "error", err)

		return handler.StatusConflict("Profession already exists")
	}

	log.Info("Profession created", "id", id, "name", profession.Name)

	handler.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":   id,
		"name": profession.Name,
	})

	return nil
}

func (h *ProfessionAdminHandler) Change(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professionID, err := handler.PathUUID(r, "id")
	if err != nil {
		log.Error("Failed to parse id", "error", err)
		return err
	}

	var req request.UpdateProfessionRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		log.Warn("Failed to decode update profession request", "error", err)
		return err
	}

	profession := entity.Profession{
		ID:           professionID,
		Name:         req.Name,
		VacancyQuery: req.VacancyQuery,
		IsActive:     req.IsActive,
	}

	if err := h.profession.ChangeProfession(ctx, profession); err != nil {
		log.Error("Failed to update profession", "id", professionID, "error", err)
		return handler.StatusInternalServerError("Failed to change profession")
	}

	handler.RespondJSON(w, http.StatusOK, response.ProfessionResponse{
		ID:           profession.ID.String(),
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
	})

	return nil
}

func (h *ProfessionAdminHandler) ListAllProfessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professions, err := h.profession.AllProfessions(ctx)
	if err != nil {
		log.Error("Failed to get all professions", "error", err)

		return handler.StatusInternalServerError("Failed to get all professions")
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
