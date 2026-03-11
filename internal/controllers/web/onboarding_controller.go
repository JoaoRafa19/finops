package web

import (
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/templates"
	"net/http"
)

type OnboardingController struct {
	workspaceService service.WorkspaceService
}

func NewOnboardingController(service service.WorkspaceService) *OnboardingController {
	return &OnboardingController{
		workspaceService: service,
	}
}

func (c *OnboardingController) Page(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	exists, err := c.workspaceService.ExistsForUser(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "failed to check workspace", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	renderTempl(w, r, http.StatusOK, templates.OnboardingPage(""))
}

func (c *OnboardingController) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		renderTempl(w, r, http.StatusBadRequest, templates.OnboardingPage("dados invalidos"))
		return
	}

	_, err := c.workspaceService.CreateWorkspace(r.Context(), service.CreateWorkspaceDTO{
		UserID:          session.UserID,
		Name:            r.FormValue("name"),
		DefaultCurrency: "BRL",
	})

	if err != nil {
		renderTempl(w, r, http.StatusInternalServerError, templates.OnboardingPage("Não foi possivel criar o workspace"))
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
