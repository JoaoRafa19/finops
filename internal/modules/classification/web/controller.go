package web

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"strconv"
	"strings"
)

type ClassificationController struct {
	classifySvc service.ClassificationService
	categorySvc service.CategoryService
}

func NewClassificationController(classifySvc service.ClassificationService, categorySvc service.CategoryService) *ClassificationController {
	return &ClassificationController{classifySvc: classifySvc, categorySvc: categorySvc}
}

func (c *ClassificationController) Page(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	txs, err := c.classifySvc.ListUnclassified(r.Context(), session.UserID, 20)
	if err != nil {
		logger.Error("classification_page_list_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao carregar transações", http.StatusInternalServerError)
		return
	}

	count, _ := c.classifySvc.CountUnclassified(r.Context(), session.UserID)

	render.Templ(w, r, http.StatusOK, templates.ClassificationPage(txs, count, session.CSRFToken))
}

func (c *ClassificationController) SuggestModal(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	txID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	suggestion, err := c.classifySvc.SuggestCategory(r.Context(), session.UserID, txID)
	if err != nil {
		logger.Warn("classification_suggest_failed", "user_id", session.UserID, "tx_id", txID, "error", err)
		suggestion = ""
	}

	cats, err := c.categorySvc.GetCategories(r.Context(), session.UserID)
	if err != nil {
		logger.Error("classification_modal_cats_failed", "user_id", session.UserID, "error", err)
	}

	// Resolve o ID da categoria sugerida para pré-popular o hidden field
	var suggestedCatID int64
	if suggestion != "" && suggestion != "Sem categoria" {
		for _, cat := range cats {
			if strings.EqualFold(cat.Name, suggestion) {
				suggestedCatID = cat.ID
				break
			}
		}
	}

	description := r.URL.Query().Get("description")
	amount := r.URL.Query().Get("amount")
	direction := r.URL.Query().Get("direction")

	render.Templ(w, r, http.StatusOK, templates.ClassificationModal(txID, description, amount, direction, suggestion, suggestedCatID, cats, session.CSRFToken))
}

func (c *ClassificationController) Classify(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "requisição inválida", http.StatusBadRequest)
		return
	}

	txID, err := strconv.ParseInt(r.FormValue("tx_id"), 10, 64)
	if err != nil {
		http.Error(w, "tx_id inválido", http.StatusBadRequest)
		return
	}

	categoryID, err := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	if err != nil || categoryID == 0 {
		render.Templ(w, r, http.StatusOK, templates.ClassificationModalError("Selecione uma categoria."))
		return
	}

	description := r.FormValue("description")

	affected, err := c.classifySvc.Classify(r.Context(), session.UserID, txID, categoryID, description)
	if err != nil {
		logger.Error("classification_classify_failed", "user_id", session.UserID, "tx_id", txID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ClassificationModalError("Erro ao classificar: "+err.Error()))
		return
	}

	logger.Info("classification_classify_succeeded", "user_id", session.UserID, "tx_id", txID, "category_id", categoryID, "auto_classified", affected)
	render.Templ(w, r, http.StatusOK, templates.ClassificationSuccess(txID, affected))
}

func (c *ClassificationController) SearchCategories(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query().Get("q")
	results, err := c.classifySvc.SearchCategories(r.Context(), session.UserID, q)
	if err != nil {
		http.Error(w, "erro", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.CategorySearchResults(results))
}

func (c *ClassificationController) AutoClassify(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	suggestions, cats, err := c.classifySvc.BulkSuggest(r.Context(), session.UserID, 10)
	if err != nil {
		logger.Error("auto_classify_suggest_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ClassificationModalError("Erro ao processar sugestões: "+err.Error()))
		return
	}

	render.Templ(w, r, http.StatusOK, templates.BulkClassificationFragment(suggestions, cats, session.CSRFToken))
}

func (c *ClassificationController) BulkConfirm(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "requisição inválida", http.StatusBadRequest)
		return
	}

	// Lê pares tx_ids (lista) + cat_{txID} (valor)
	txIDs := r.Form["tx_ids"]
	classifications := make(map[int64]int64, len(txIDs))
	for _, idStr := range txIDs {
		txID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		catStr := r.FormValue("cat_" + idStr)
		catID, err := strconv.ParseInt(catStr, 10, 64)
		if err != nil || catID == 0 {
			continue
		}
		classifications[txID] = catID
	}

	confirmed, err := c.classifySvc.BulkConfirm(r.Context(), session.UserID, classifications)
	if err != nil {
		logger.Error("bulk_confirm_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusInternalServerError, templates.ClassificationModalError("Erro ao confirmar: "+err.Error()))
		return
	}

	logger.Info("bulk_confirm_succeeded", "user_id", session.UserID, "confirmed", confirmed)
	w.Header().Set("HX-Redirect", "/classify")
	w.WriteHeader(http.StatusOK)
}

func (c *ClassificationController) NotificationsCount(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		render.Templ(w, r, http.StatusOK, templates.PendingBadge(0))
		return
	}
	count, _ := c.classifySvc.CountUnclassified(r.Context(), session.UserID)
	render.Templ(w, r, http.StatusOK, templates.PendingBadge(count))
}

func (c *ClassificationController) CreateAndClassify(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "requisição inválida", http.StatusBadRequest)
		return
	}

	txID, err := strconv.ParseInt(r.FormValue("tx_id"), 10, 64)
	if err != nil {
		http.Error(w, "tx_id inválido", http.StatusBadRequest)
		return
	}

	name := r.FormValue("new_category_name")
	kind := r.FormValue("new_category_kind")
	description := r.FormValue("description")

	if name == "" || kind == "" {
		render.Templ(w, r, http.StatusOK, templates.ClassificationModalError("Nome e tipo da categoria são obrigatórios."))
		return
	}

	cat, err := c.categorySvc.CreateCategory(r.Context(), service.CreateCategoryDTO{
		UserID: session.UserID,
		Name:   name,
		Kind:   service.CategoryKind(kind),
	})
	if err != nil {
		logger.Error("classification_create_cat_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ClassificationModalError("Erro ao criar categoria: "+err.Error()))
		return
	}

	affected, err := c.classifySvc.Classify(r.Context(), session.UserID, txID, cat.ID, description)
	if err != nil {
		logger.Error("classification_classify_after_create_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ClassificationModalError("Categoria criada mas erro ao classificar: "+err.Error()))
		return
	}

	logger.Info("classification_create_and_classify_succeeded", "user_id", session.UserID, "cat_id", cat.ID, "auto_classified", affected)
	render.Templ(w, r, http.StatusOK, templates.ClassificationSuccess(txID, affected))
}
