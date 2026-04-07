package home

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
)

type Controller struct {
	accountService     service.AccountService
	categoryService    service.CategoryService
	workspaceService   service.WorkspaceService
	transactionService service.TransactionService
}

func NewController(accountService service.AccountService, categoryService service.CategoryService, workspaceService service.WorkspaceService, transactionService service.TransactionService) *Controller {
	return &Controller{
		accountService:     accountService,
		categoryService:    categoryService,
		workspaceService:   workspaceService,
		transactionService: transactionService,
	}
}

func (c *Controller) Home(w http.ResponseWriter, r *http.Request) {
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

	accounts, err := c.accountService.ListSummariesByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("home_load_accounts_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	categories, err := c.categoryService.GetCategories(r.Context(), session.UserID)
	if err != nil {
		logger.Error("home_load_categories_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}

	transactions, err := c.transactionService.ListRecentByUser(r.Context(), session.UserID, 10)
	if err != nil {
		logger.Error("home_load_transactions_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load transactions", http.StatusInternalServerError)
		return
	}

	transactions_dto := make([]templates.TransactionDTO, len(transactions))
	for i, t := range transactions {
		transactions_dto[i] = templates.TransactionDTO{
			ID:          int(t.ID),
			Description: t.Description,
			Amount:      t.Amount,
			Category:    t.Category,
			CreatedAt:   t.PostedOn,
			Account:     t.AccountName,
			Direction:   t.Direction,
			Currency:    t.Currency,
		}
	}

	accountsDTO := make([]templates.AccountDTO, len(accounts))
	for i, acc := range accounts {
		accountsDTO[i] = templates.AccountDTO{
			ID:             int64(acc.ID),
			Name:           acc.Name,
			Type:           acc.Type,
			Currency:       acc.Currency,
			OpeningBalance: acc.OpeningBalance,
			CurrentBalance: acc.CurrentBalance,
			OpeningDate:    "",
		}
	}

	logger.Debug("home_rendered", "user_id", session.UserID, "accounts_count", len(accounts), "categories_count", len(categories))
	render.Templ(w, r, http.StatusOK, templates.HomePage(session.Email, session.CSRFToken, accountsDTO, categories, transactions_dto))
}
