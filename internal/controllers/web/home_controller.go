package web

import (
	"net/http"

	"finops/internal/web/middleware"
	"finops/internal/web/templates"
)

type HomeController struct{}

func NewHomeController() *HomeController {
	return &HomeController{}
}

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderTempl(w, r, http.StatusOK, templates.HomePage(session.Email, session.CSRFToken))
}
