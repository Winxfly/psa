package admin

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/internal/handler/http/v1/request"
	"psa/internal/handler/http/v1/response"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

type ProfessionAdminAccesser interface {
	AllProfessions(ctx context.Context) ([]domain.Profession, error)
	CreateProfession(ctx context.Context, profession domain.Profession) (uuid.UUID, error)
	ChangeProfession(ctx context.Context, profession domain.Profession) error
}

type ProfessionAdminHandler struct {
	profession ProfessionAdminAccesser
}

func NewProfessionAdminHandler(profession ProfessionAdminAccesser) *ProfessionAdminHandler {
	return &ProfessionAdminHandler{
		profession: profession,
	}
}

func (h *ProfessionAdminHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	var req request.CreateProfessionRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		log.Warn("profession.admin.create.decode_failed", slogx.Err(err))

		return err
	}

	profession := domain.Profession{
		Name:         req.Name,
		VacancyQuery: req.VacancyQuery,
		IsActive:     true,
	}

	id, err := h.profession.CreateProfession(ctx, profession)
	if err != nil {
		log.Warn("profession.admin.create.conflict", "name", profession.Name)

		return handler.StatusConflict("Profession already exists")
	}

	log.Info("profession.admin.create.success", "profession_id", id, "name", profession.Name)

	handler.RespondJSON(w, http.StatusCreated, response.ProfessionAdminResponse{
		ID:           id.String(),
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})

	return nil
}

func (h *ProfessionAdminHandler) Change(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professionID, err := handler.PathUUID(r, "id")
	if err != nil {
		log.Warn("profession.admin.change.invalid_id", slogx.Err(err))
		return err
	}

	var req request.UpdateProfessionRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		log.Warn("profession.admin.change.decode_failed", slogx.Err(err))
		return err
	}

	profession := domain.Profession{
		ID:           professionID,
		Name:         req.Name,
		VacancyQuery: req.VacancyQuery,
		IsActive:     req.IsActive,
	}

	if err := h.profession.ChangeProfession(ctx, profession); err != nil {
		log.Error("profession.admin.change.failed", "profession_id", professionID, slogx.Err(err))
		return handler.StatusInternalServerError("Failed to change profession")
	}

	log.Info("profession.admin.change.success", "profession_id", professionID)

	handler.RespondJSON(w, http.StatusOK, response.ProfessionAdminResponse{
		ID:           profession.ID.String(),
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})

	return nil
}

func (h *ProfessionAdminHandler) ListAllProfessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professions, err := h.profession.AllProfessions(ctx)
	if err != nil {
		log.Error("profession.admin.list.failed", slogx.Err(err))

		return handler.StatusInternalServerError("Failed to get all professions")
	}

	resp := make([]response.ProfessionAdminResponse, len(professions))
	for i, p := range professions {
		resp[i] = response.ProfessionAdminResponse{
			ID:           p.ID.String(),
			Name:         p.Name,
			VacancyQuery: p.VacancyQuery,
			IsActive:     p.IsActive,
		}
	}

	log.Debug("profession.admin.list.success", "count", len(professions))

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}
