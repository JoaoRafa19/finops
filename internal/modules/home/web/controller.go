package home

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
)

type Controller struct {
	accountService   service.AccountService
	workspaceService service.WorkspaceService
}

func NewController(accountService service.AccountService, workspaceService service.WorkspaceService) *Controller {
	return &Controller{
		accountService:   accountService,
		workspaceService: workspaceService,
	}
}

func (c *Controller) Home(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("home_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	exists, err := c.workspaceService.ExistsForUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("home_workspace_check_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load workspace", http.StatusInternalServerError)
		return
	}

	if !exists {
		logger.Debug("home_redirect_onboarding", "user_id", session.UserID)
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}

	accounts, err := c.accountService.ListByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("home_load_accounts_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	logger.Debug("home_rendered", "user_id", session.UserID, "accounts_count", len(accounts))
	render.Templ(w, r, http.StatusOK, templates.HomePage(session.Email, session.CSRFToken, accounts))
}
