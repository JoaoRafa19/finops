package web

import (
	"database/sql"
	"errors"
	"net/http"

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

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accounts, err := c.accountService.ListByUser(r.Context(), session.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		}
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderTempl(w, r, http.StatusOK, templates.HomePage(session.Email, session.CSRFToken, accounts))
}
