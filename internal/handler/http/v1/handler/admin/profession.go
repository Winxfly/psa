package admin

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/pkg/logger/loggerctx"
	"psa/pkg/logger/slogx"
)

type ProfessionAdminAccesser interface {
	AllProfessions(ctx context.Context) ([]domain.Profession, error)
	CreateProfession(ctx context.Context, profession domain.Profession) (uuid.UUID, error)
	ChangeProfession(ctx context.Context, profession domain.Profession) error
}

type ScrapingProvider interface {
	ProcessActiveProfessionsArchive(ctx context.Context) error
	ProcessActiveProfessionsDaily(ctx context.Context) error
}

type ProfessionAdminHandler struct {
	profession         ProfessionAdminAccesser
	scraping           ScrapingProvider
	scrapingInProgress atomic.Bool
}

func NewProfessionAdminHandler(profession ProfessionAdminAccesser, scraping ScrapingProvider) *ProfessionAdminHandler {
	return &ProfessionAdminHandler{
		profession: profession,
		scraping:   scraping,
	}
}

type createProfessionRequest struct {
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
}

type professionAdminResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
	IsActive     bool   `json:"is_active"`
}

func (h *ProfessionAdminHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	var req createProfessionRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		log.Warn("profession_admin_create_decode_failed", slogx.Err(err))

		return err
	}

	if strings.TrimSpace(req.Name) == "" {
		return handler.StatusBadRequest("Name is required")
	}
	if strings.TrimSpace(req.VacancyQuery) == "" {
		return handler.StatusBadRequest("Vacancy query is required")
	}

	profession := domain.Profession{
		Name:         req.Name,
		VacancyQuery: req.VacancyQuery,
		IsActive:     true,
	}

	id, err := h.profession.CreateProfession(ctx, profession)
	if err != nil {
		if errors.Is(err, domain.ErrProfessionAlreadyExists) {
			log.Warn("profession_admin_create_conflict", "name", profession.Name)
			return handler.StatusConflict("Profession already exists")
		}

		log.Error("profession_admin_create_failed", "name", profession.Name, slogx.Err(err))
		return handler.StatusInternalServerError("Failed to create profession")
	}

	log.Info("profession_admin_create_success", "profession_id", id, "name", profession.Name)

	handler.RespondJSON(w, http.StatusCreated, professionAdminResponse{
		ID:           id.String(),
		Name:         profession.Name,
		VacancyQuery: profession.VacancyQuery,
		IsActive:     profession.IsActive,
	})

	return nil
}

type updateProfessionRequest struct {
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
	IsActive     bool   `json:"is_active"`
}

func (h *ProfessionAdminHandler) Change(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	log := loggerctx.FromContext(ctx)

	professionID, err := handler.PathUUID(r, "id")
	if err != nil {
		log.Warn("profession_admin_change_invalid_id", slogx.Err(err))
		return err
	}

	var req updateProfessionRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		log.Warn("profession_admin_change_decode_failed", slogx.Err(err))
		return err
	}

	profession := domain.Profession{
		ID:           professionID,
		Name:         req.Name,
		VacancyQuery: req.VacancyQuery,
		IsActive:     req.IsActive,
	}

	if err := h.profession.ChangeProfession(ctx, profession); err != nil {
		log.Error("profession_admin_change_failed", "profession_id", professionID, slogx.Err(err))
		return handler.StatusInternalServerError("Failed to change profession")
	}

	log.Info("profession_admin_change_success", "profession_id", professionID)

	handler.RespondJSON(w, http.StatusOK, professionAdminResponse{
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
		log.Error("profession_admin_list_failed", slogx.Err(err))

		return handler.StatusInternalServerError("Failed to get all professions")
	}

	resp := make([]professionAdminResponse, len(professions))
	for i, p := range professions {
		resp[i] = professionAdminResponse{
			ID:           p.ID.String(),
			Name:         p.Name,
			VacancyQuery: p.VacancyQuery,
			IsActive:     p.IsActive,
		}
	}

	log.Debug("profession_admin_list_success", "count", len(professions))

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ProfessionAdminHandler) triggerScraping(
	w http.ResponseWriter, r *http.Request,
	mode string, runFunc func(context.Context) error) error {
	log := loggerctx.FromContext(r.Context()).With("component", "scraping")

	if !h.scrapingInProgress.CompareAndSwap(false, true) {
		return handler.StatusConflict("Scraping already in progress")
	}

	log.Info("scraping_triggered", "mode", mode)

	go func() {
		defer h.scrapingInProgress.Store(false)
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("scraping_panic", "mode", mode, "panic", rec, "stack", string(debug.Stack()))
			}
		}()

		ctx := loggerctx.WithLogger(context.Background(), log)
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()

		taskLog := loggerctx.FromContext(ctx)

		if err := runFunc(ctx); err != nil {
			taskLog.Error("scraping_failed", "mode", mode, slogx.Err(err))
			return
		}

		taskLog.Info("scraping_completed", "mode", mode)
	}()

	handler.RespondJSON(w, http.StatusAccepted, map[string]string{
		"status": "started",
		"mode":   mode,
	})

	return nil
}

func (h *ProfessionAdminHandler) TriggerArchiveScraping(w http.ResponseWriter, r *http.Request) error {
	return h.triggerScraping(w, r, "archive", h.scraping.ProcessActiveProfessionsArchive)
}

func (h *ProfessionAdminHandler) TriggerCacheScraping(w http.ResponseWriter, r *http.Request) error {
	return h.triggerScraping(w, r, "cache", h.scraping.ProcessActiveProfessionsDaily)
}
