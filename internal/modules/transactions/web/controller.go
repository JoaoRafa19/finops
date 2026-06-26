package transactions

import (
	"database/sql"
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

type transferPayload struct {
	FromAccountID int64
	ToAccountID   int64
	PostedOn      time.Time
	Amount        float64
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
		c.renderTransactionsModal(w, r, session.UserID, session.CSRFToken, buildTransactionFormState(r, err.Error()))
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
		c.renderTransactionsModal(w, r, session.UserID, session.CSRFToken, buildTransactionFormState(r, "erro ao criar transação"))
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

	render.Templ(w, r, http.StatusOK, templates.TransactionModalDialog(templates.NewTransactionFormState(), session.CSRFToken, accounts, categories))
}

func (c *Controller) RegisterTransferModal(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("register_transfer_unauthorized")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	accounts, err := c.accountService.ListByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("register_transfer_accounts_lookup_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	if len(accounts) < 2 {
		render.Templ(w, r, http.StatusBadRequest, templates.TransferModalBlocked())
		return
	}

	render.Templ(w, r, http.StatusOK, templates.TransferModalDialog(templates.NewTransferFormState(), session.CSRFToken, accounts))
}

func (c *Controller) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("create_transfer_unauthorized")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	payload, status, err := parseTransferPayload(r)

	if payload.FromAccountID == payload.ToAccountID {
		logger.Warn("create_transfer_invalid_payload_same_account", "user_id", session.UserID)
		c.renderTransferModal(w, r, session.UserID, session.CSRFToken, buildTransferFormState(r, "contas de origem e destino devem ser diferentes"))
		return
	}

	if err != nil {
		logger.Warn("create_transfer_invalid_payload", "user_id", session.UserID, "status", status, "error", err)
		c.renderTransferModal(w, r, session.UserID, session.CSRFToken, buildTransferFormState(r, err.Error()))
		return
	}

	_, err = c.transactionService.CreateTransfer(r.Context(), models.CreateTransferDTO{
		UserID:        session.UserID,
		FromAccountID: payload.FromAccountID,
		ToAccountID:   payload.ToAccountID,
		PostedOn:      payload.PostedOn,
		Amount:        payload.Amount,
	})
	if err != nil {
		logger.Error("create_transfer_failed", "user_id", session.UserID, "error", err)
		c.renderTransferModal(w, r, session.UserID, session.CSRFToken, buildTransferFormState(r, err.Error()))
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) EditModal(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	txID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	tx, err := c.transactionService.GetForEdit(r.Context(), session.UserID, txID)
	if err != nil {
		logger.Error("edit_modal_lookup_failed", "tx_id", txID, "error", err)
		http.Error(w, "transaction not found", http.StatusNotFound)
		return
	}

	if tx.TransferGroupID.Valid {
		http.Error(w, "transfers cannot be edited", http.StatusBadRequest)
		return
	}

	accounts, err := c.accountService.ListByUser(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}
	categories, err := c.categoryService.GetCategories(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}

	amount, _ := strconv.ParseFloat(tx.Amount, 64)
	state := templates.TransactionFormState{
		Description: tx.Description,
		Amount:      strconv.FormatFloat(amount, 'f', 2, 64),
		AccountID:   strconv.FormatInt(tx.AccountID, 10),
		CategoryID:  "",
		PostedOn:    tx.PostedOn.Format("2006-01-02"),
		Direction:   tx.Direction,
	}
	if tx.CategoryID.Valid {
		state.CategoryID = strconv.FormatInt(tx.CategoryID.Int64, 10)
	}

	render.Templ(w, r, http.StatusOK, templates.TransactionEditModalDialog(txID, state, session.CSRFToken, accounts, categories))
}

func (c *Controller) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	txID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	payload, _, err := parseTransactionPayload(r)
	if err != nil {
		c.renderEditModal(w, r, session.UserID, txID, session.CSRFToken, buildTransactionFormState(r, err.Error()))
		return
	}

	var catID sql.NullInt64
	if payload.CategoryID != nil {
		catID = sql.NullInt64{Int64: *payload.CategoryID, Valid: true}
	}

	if err := c.transactionService.Update(r.Context(), session.UserID, txID, service.UpdateTransactionDTO{
		Description: payload.Description,
		Amount:      payload.Amount,
		Direction:   payload.Direction,
		PostedOn:    payload.PostedOn,
		CategoryID:  catID,
		AccountID:   payload.AccountID,
	}); err != nil {
		logger.Error("update_transaction_failed", "tx_id", txID, "error", err)
		c.renderEditModal(w, r, session.UserID, txID, session.CSRFToken, buildTransactionFormState(r, "erro ao atualizar transação"))
		return
	}

	logger.Info("transaction_updated", "user_id", session.UserID, "tx_id", txID)
	w.Header().Set("HX-Redirect", r.Header.Get("Referer"))
	if w.Header().Get("HX-Redirect") == "" {
		w.Header().Set("HX-Redirect", "/")
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	txID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := c.transactionService.Delete(r.Context(), session.UserID, txID); err != nil {
		logger.Error("delete_transaction_failed", "tx_id", txID, "error", err)
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}

	logger.Info("transaction_deleted", "user_id", session.UserID, "tx_id", txID)
	w.WriteHeader(http.StatusOK) // HTMX outerHTML swap to empty removes the element
}

func (c *Controller) renderEditModal(w http.ResponseWriter, r *http.Request, userID, txID int64, csrf string, form templates.TransactionFormState) {
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
	render.Templ(w, r, http.StatusBadRequest, templates.TransactionEditModalDialog(txID, form, csrf, accounts, categories))
}

func (c *Controller) renderTransactionsModal(w http.ResponseWriter, r *http.Request, userID int64, csrfToken string, form templates.TransactionFormState) {
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

	render.Templ(w, r, http.StatusBadRequest, templates.TransactionModalDialog(form, csrfToken, accounts, categories))
}

func (c *Controller) renderTransferModal(w http.ResponseWriter, r *http.Request, userID int64, csrfToken string, form templates.TransferFormState) {
	accounts, err := c.accountService.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.TransferModalDialog(form, csrfToken, accounts))
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

func parseTransferPayload(r *http.Request) (transferPayload, int, error) {
	if err := r.ParseForm(); err != nil {
		return transferPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	fromAccountID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("from_account_id")), 10, 64)
	if err != nil {
		return transferPayload{}, http.StatusBadRequest, errors.New("invalid from_account_id")
	}

	toAccountID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("to_account_id")), 10, 64)
	if err != nil {
		return transferPayload{}, http.StatusBadRequest, errors.New("invalid to_account_id")
	}

	if fromAccountID == toAccountID {
		return transferPayload{}, http.StatusBadRequest, errors.New("source and destination accounts must be different")
	}

	amountValue := normalizeAmount(r.FormValue("amount"))
	if amountValue == "" {
		return transferPayload{}, http.StatusBadRequest, errors.New("amount must be a valid number")
	}

	amountValueFloat, err := strconv.ParseFloat(strings.TrimSpace(amountValue), 64)
	if err != nil || amountValueFloat <= 0 {
		return transferPayload{}, http.StatusBadRequest, errors.New("amount must be a valid number")
	}

	postedOnValue := strings.TrimSpace(r.FormValue("posted_on"))
	if postedOnValue == "" {
		return transferPayload{}, http.StatusBadRequest, errors.New("posted_on is required")
	}

	postedOn, err := time.Parse("2006-01-02", postedOnValue)
	if err != nil {
		return transferPayload{}, http.StatusBadRequest, errors.New("posted_on must be in the format YYYY-MM-DD")
	}

	return transferPayload{
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		PostedOn:      postedOn,
		Amount:        amountValueFloat,
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

func buildTransactionFormState(r *http.Request, errorMessage string) templates.TransactionFormState {
	return templates.NewTransactionFormStateFromValues(
		r.FormValue("description"),
		r.FormValue("amount"),
		r.FormValue("account_id"),
		r.FormValue("category_id"),
		r.FormValue("posted_on"),
		r.FormValue("direction"),
		errorMessage,
	)
}

func buildTransferFormState(r *http.Request, errorMessage string) templates.TransferFormState {
	return templates.NewTransferFormStateFromValues(
		r.FormValue("from_account_id"),
		r.FormValue("to_account_id"),
		r.FormValue("amount"),
		r.FormValue("posted_on"),
		errorMessage,
	)
}
