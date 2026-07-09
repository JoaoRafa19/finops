package onboarding

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
)

type Controller struct {
	workspaceService service.WorkspaceService
	settingsService  service.UserSettingsService
}

func NewController(wsService service.WorkspaceService, settingsService service.UserSettingsService) *Controller {
	return &Controller{
		workspaceService: wsService,
		settingsService:  settingsService,
	}
}

func (c *Controller) Page(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("onboarding_page_unauthenticated")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	exists, err := c.workspaceService.ExistsForUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("onboarding_workspace_check_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to check workspace", http.StatusInternalServerError)
		return
	}

	if exists {
		logger.Debug("onboarding_redirect_home", "user_id", session.UserID)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	logger.Debug("onboarding_page_rendered", "user_id", session.UserID)
	render.Templ(w, r, http.StatusOK, templates.OnboardingPage("", session.CSRFToken))
}

func (c *Controller) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("onboarding_create_unauthenticated")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Warn("onboarding_create_invalid_form", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusBadRequest, templates.OnboardingPage("dados invalidos", session.CSRFToken))
		return
	}

	_, err := c.workspaceService.CreateWorkspace(r.Context(), service.CreateWorkspaceDTO{
		UserID:          session.UserID,
		Name:            r.FormValue("name"),
		DefaultCurrency: "BRL",
	})
	if err != nil {
		logger.Error("onboarding_create_workspace_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusInternalServerError, templates.OnboardingPage("Não foi possivel criar o workspace", session.CSRFToken))
		return
	}

	// Modo do dashboard escolhido no onboarding (default simples para iniciantes).
	mode := service.HomeModeSimple
	if r.FormValue("home_mode") == service.HomeModeAdvanced {
		mode = service.HomeModeAdvanced
	}
	if err := c.settingsService.SetHomeMode(r.Context(), session.UserID, mode); err != nil {
		logger.Warn("onboarding_set_home_mode_failed", "user_id", session.UserID, "error", err)
	}

	logger.Debug("onboarding_create_workspace_succeeded", "user_id", session.UserID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
