package web

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
)

type Controller struct {
	categoryService service.CategoryService
	accountService  service.AccountService
}

func NewController(categorySvc service.CategoryService, accountSvc service.AccountService) *Controller {
	return &Controller{categoryService: categorySvc, accountService: accountSvc}
}

func (c *Controller) Page(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	categories, err := c.categoryService.GetCategories(r.Context(), session.UserID)
	if err != nil {
		logger.Warn("profile_categories_failed", "user", session.UserID, "error", err.Error())
	}

	summaries, err := c.accountService.ListSummariesByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Warn("profile_accounts_failed", "user", session.UserID, "error", err.Error())
	}
	accounts := make([]templates.AccountItemDTO, 0, len(summaries))
	for _, s := range summaries {
		accounts = append(accounts, templates.AccountItemDTO{
			ID:             s.ID,
			Name:           s.Name,
			Type:           s.Type,
			Currency:       s.Currency,
			CurrentBalance: s.CurrentBalance,
		})
	}

	render.Templ(w, r, http.StatusOK, templates.ProfilePage(session.Email, session.CSRFToken, categories, accounts))
}
