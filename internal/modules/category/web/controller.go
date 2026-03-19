package category

import (
	"encoding/json"
	"finops/internal/models"
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"fmt"
	"net/http"
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

func (c *CategoryController) CreateCategory(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())

	if !ok {
		logger.Warn("category_create_unauthorized")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	ws, err := c.wps.GetByOwnerUserID(r.Context(), session.UserID)
	if err != nil {
		logger.Warn("category_create_error_getting_workspace")
		// see handle
		return
	}

	var categoryPayload models.CreateCategoryDTO

	if err := json.NewDecoder(r.Body).Decode(&categoryPayload); err != nil {
		logger.Warn("category_create_bad_request")
		// see handle
		return
	}

	cat, err := c.categoryService.CreateCategory(r.Context(), service.CreateCategoryDTO{
		Kind:        service.CategoryKind(categoryPayload.Kind),
		UserID:      session.UserID,
		WorkspaceID: ws.ID,
		Name:        categoryPayload.Name,
	})

	if err != nil {
		logger.Warn("category_create_error")
		return 
	}

	//return render template
	fmt.Println(cat)
	
}
