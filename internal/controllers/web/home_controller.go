package web

import (
	"finops/internal/observability"
	"net/http"
	"strconv"

	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/templates"
)

type HomeController struct {
	accountService   service.AccountService
	workspaceService service.WorkspaceService
}

func NewHomeController(accountService service.AccountService, workspaceService service.WorkspaceService) *HomeController {
	return &HomeController{
		accountService:   accountService,
		workspaceService: workspaceService,
	}
}

func (c *AccountController) ShowItem(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_show_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid account id", http.StatusBadRequest)
		return
	}

	account, err := c.accountService.GetByID(r.Context(), session.UserID, accountID)
	if err != nil {
		logger.Error("account_show_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "failed to load account", http.StatusInternalServerError)
		return
	}

	renderTempl(w, r, http.StatusOK, templates.AccountItem(account))
}

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderTempl(w, r, http.StatusOK, templates.HomePage(session.Email, session.CSRFToken, accounts))
}
