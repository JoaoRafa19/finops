package category

import (
	"finops/internal/models"
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/utils"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type CategoryController struct {
	categoryService service.CategoryService
	wps             service.WorkspaceService
}

func NewCategoryController(serv service.CategoryService) *CategoryController {
	return &CategoryController{
		categoryService: serv,
	}
}

func (c *CategoryController) GetCategories(w http.ResponseWriter, r *http.Request) {

	userID := r.Context().Value("user")
	if userID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userid, err := strconv.ParseInt(userID.(string), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	cat, err := c.categoryService.GetCategories(r.Context(), userid)
	if err != nil {
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}

	categories := make([]models.CategoryResponse, len(cat))
	for i, c := range cat {
		categories[i] = models.CategoryResponse{
			ID:   c.ID,
			Name: c.Name,
			Kind: string(c.Kind),
		}
	}

	response := struct {
		Categories []models.CategoryResponse `json:"categories"`
	}{
		Categories: categories,
	}

	utils.WriteJSONResponse(w, 200, response)

}

func parseCategoryPayload(r *http.Request) (models.CreateCategoryDTO, int, error) {
	if err := r.ParseForm(); err != nil {
		return models.CreateCategoryDTO{}, http.StatusBadRequest, fmt.Errorf("failed to parse form: %w", err)
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		return models.CreateCategoryDTO{}, http.StatusBadRequest, fmt.Errorf("name is required")
	}
	kind := service.CategoryKind(strings.TrimSpace(r.FormValue("category_kind")))
	switch kind {
	case service.EXPENSE, service.INCOME, service.TRANSFER:
		// valid kind
	default:
		return models.CreateCategoryDTO{}, http.StatusBadRequest, fmt.Errorf("invalid kind: must be 'expense', 'income', or 'transfer'")
	}
	return models.CreateCategoryDTO{
		Name: name,
		Kind: models.CategoryKind(kind),
	}, http.StatusOK, nil
}

func (c *CategoryController) CreateCategory(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())

	if !ok {
		logger.Warn("category_create_unauthorized")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	categoryPayload, _, err := parseCategoryPayload(r)
	if err != nil {
		logger.Warn("category_create_invalid_payload", "user", session.UserID, "error", err.Error())
		c.renderCategoryPannel(w, r, session.UserID, session.CSRFToken, buildCategoryFormState(r, err.Error()))
		return
	}

	cat, err := c.categoryService.CreateCategory(r.Context(), service.CreateCategoryDTO{
		Kind:   service.CategoryKind(categoryPayload.Kind),
		UserID: session.UserID,
		Name:   categoryPayload.Name,
	})

	if err != nil {
		logger.Warn("category_create_error", "user", session.UserID, "error", err.Error())
		c.renderCategoryPannel(w, r, session.UserID, session.CSRFToken, buildCategoryFormState(r, err.Error()))
		return
	}

	logger.Info("category_create_success", "user", session.UserID, "category", cat.ID)
	c.renderCategoryPannel(w, r, session.UserID, session.CSRFToken, templates.NewCategoryFormState())

}

func buildCategoryFormState(r *http.Request, errMsg string) templates.CategoryFormState {
	name := r.FormValue("name")
	kind := r.FormValue("category_kind")

	return templates.CategoryFormState{
		Name:         name,
		Kind:         kind,
		ErrorMessage: errMsg,
	}
}

func (c *CategoryController) renderCategoryPannel(w http.ResponseWriter, r *http.Request, userID int64, token string, form templates.CategoryFormState) {

	categories, err := c.categoryService.GetCategories(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.CategoryPannels(token, categories, form))
}
