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
}

func NewController(categorySvc service.CategoryService) *Controller {
	return &Controller{categoryService: categorySvc}
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

	render.Templ(w, r, http.StatusOK, templates.ProfilePage(session.Email, session.CSRFToken, categories))
}
