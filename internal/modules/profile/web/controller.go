package web

import (
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
)

type Controller struct{}

func NewController() *Controller { return &Controller{} }

func (c *Controller) Page(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.ProfilePage(session.Email, session.CSRFToken))
}
