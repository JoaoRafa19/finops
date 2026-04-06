package transactions

import (
	"errors"
	"finops/internal/models"
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Controller struct {
	transactionService service.TransactionService
	accountService     service.AccountService
	categoryService    service.CategoryService
}

type transactionPayload struct {
	AccountID   int64
	CategoryID  *int64
	PostedOn    time.Time
	Description string
	Amount      float64
	Direction   string
}

func NewController(
	transactionService service.TransactionService,
	accountService service.AccountService,
	categoryService service.CategoryService,
) *Controller {
	return &Controller{
		transactionService: transactionService,
		accountService:     accountService,
		categoryService:    categoryService,
	}
}

func (c *Controller) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("create_transaction_unauthorized")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	payload, status, err := parseTransactionPayload(r)
	if err != nil {
		logger.Warn("create_transaction_invalid_payload", "user_id", session.UserID, "status", status, "error", err)
		c.renderTransactionsModal(w, r, session.UserID, session.CSRFToken, err.Error())
		return
	}

	t, err := c.transactionService.CreateManual(r.Context(), models.CreateTransactionDTO{
		UserID:      session.UserID,
		AccountID:   payload.AccountID,
		CategoryID:  payload.CategoryID,
		PostedOn:    payload.PostedOn,
		Description: payload.Description,
		Amount:      payload.Amount,
		Direction:   payload.Direction,
	})
	if err != nil {
		logger.Error("create_transaction_failed", "user_id", session.UserID, "error", err)
		c.renderTransactionsModal(w, r, session.UserID, session.CSRFToken, "Erro ao criar transacao")
		return
	}

	logger.Info("transaction_created", "user_id", session.UserID, "transaction_id", t.ID)
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) RegisterTransactionModal(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("register_transaction_unauthorized")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	accounts, err := c.accountService.ListByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("register_transaction_accounts_lookup_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	categories, err := c.categoryService.GetCategories(r.Context(), session.UserID)
	if err != nil {
		logger.Error("register_transaction_categories_lookup_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}

	if len(accounts) == 0 {
		render.Templ(w, r, http.StatusBadRequest, templates.TransactionModalBlocked())
		return
	}

	render.Templ(w, r, http.StatusOK, templates.TransactionModalDialog(session.CSRFToken, "", accounts, categories))
}

func (c *Controller) renderTransactionsModal(w http.ResponseWriter, r *http.Request, userID int64, csrfToken, errMsg string) {
	accounts, err := c.accountService.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	categories, err := c.categoryService.GetCategories(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusBadRequest, templates.TransactionModalDialog(csrfToken, errMsg, accounts, categories))
}

func parseTransactionPayload(r *http.Request) (transactionPayload, int, error) {
	if err := r.ParseForm(); err != nil {
		return transactionPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	accountID := r.FormValue("account_id")
	if accountID == "" {
		return transactionPayload{}, http.StatusBadRequest, errors.New("account_id is required")
	}

	categoryIDValue := strings.TrimSpace(r.FormValue("category_id"))

	amountValue := normalizeAmount(r.FormValue("amount"))
	if amountValue == "" {
		return transactionPayload{}, http.StatusBadRequest, errors.New("amount must be a valid number")
	}

	postedOnValue := strings.TrimSpace(r.FormValue("posted_on"))
	if postedOnValue == "" {
		return transactionPayload{}, http.StatusBadRequest, errors.New("posted_on is required")
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		return transactionPayload{}, http.StatusBadRequest, errors.New("description is required")
	}

	direction := strings.TrimSpace(r.FormValue("direction"))
	if direction != "credit" && direction != "debit" {
		return transactionPayload{}, http.StatusBadRequest, errors.New("direction must be either 'credit' or 'debit'")
	}

	account, err := strconv.ParseInt(accountID, 10, 64)
	if err != nil {
		return transactionPayload{}, http.StatusBadRequest, errors.New("invalid account_id")
	}

	var categoryID *int64
	if categoryIDValue != "" {
		parsedCategoryID, err := strconv.ParseInt(categoryIDValue, 10, 64)
		if err != nil {
			return transactionPayload{}, http.StatusBadRequest, errors.New("invalid category_id")
		}
		categoryID = &parsedCategoryID
	}

	parsedDate, err := time.Parse("2006-01-02", postedOnValue)
	if err != nil {
		return transactionPayload{}, http.StatusBadRequest, errors.New("posted_on must be in the format YYYY-MM-DD")
	}

	amountValueFloat, err := strconv.ParseFloat(strings.TrimSpace(amountValue), 64)
	if err != nil || amountValueFloat <= 0 {
		return transactionPayload{}, http.StatusBadRequest, errors.New("amount must be a valid number")
	}

	return transactionPayload{
		AccountID:   account,
		CategoryID:  categoryID,
		PostedOn:    parsedDate,
		Description: description,
		Amount:      amountValueFloat,
		Direction:   direction,
	}, http.StatusOK, nil
}

func normalizeAmount(value string) string {
	amount := strings.TrimSpace(value)
	if amount == "" {
		return ""
	}

	if strings.Count(amount, ",") == 1 && !strings.Contains(amount, ".") {
		return strings.Replace(amount, ",", ".", 1)
	}

	return amount
}
