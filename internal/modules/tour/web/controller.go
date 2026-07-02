package web

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
)

type TourController struct {
	tourSvc service.TourService
}

func NewTourController(tourSvc service.TourService) *TourController {
	return &TourController{tourSvc: tourSvc}
}

func (c *TourController) Page(w http.ResponseWriter, r *http.Request) {
	render.Templ(w, r, http.StatusOK, templates.TourPage())
}

func (c *TourController) Start(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Tour usa dados mock em memória; nada é persistido.
	logger.Info("tour_started", "user_id", session.UserID)
	http.Redirect(w, r, "/?tour=1", http.StatusSeeOther)
}

func (c *TourController) Complete(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := c.tourSvc.CompleteTour(r.Context(), session.UserID); err != nil {
		logger.Error("tour_complete_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao finalizar tour", http.StatusInternalServerError)
		return
	}

	logger.Info("tour_completed", "user_id", session.UserID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *TourController) Skip(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := c.tourSvc.SkipTour(r.Context(), session.UserID); err != nil {
		logger.Error("tour_skip_failed", "user_id", session.UserID, "error", err)
	}

	logger.Info("tour_skipped", "user_id", session.UserID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
