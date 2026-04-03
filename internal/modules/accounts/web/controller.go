package accounts

import (
	"errors"
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
	accountService service.AccountService
}

type accountPayload struct {
	Name           string
	Type           string
	Currency       string
	OpeningBalance float64
	OpeningDate    *time.Time
}

func NewController(accountService service.AccountService) *Controller {
	return &Controller{
		accountService: accountService,
	}
}

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_create_unauthorized")
		c.renderAccountsModal(w, r, session.UserID, session.CSRFToken, templates.NewCreateAccountFormState())
		return
	}

	payload, status, err := parseAccountPayload(r)
	if err != nil {
		logger.Warn("account_create_invalid_payload", "user_id", session.UserID, "status", status, "error", err)
		c.renderAccountsModal(w, r, session.UserID, session.CSRFToken, buildCreateAccountFormState(r, err.Error()))
		return
	}

	_, err = c.accountService.Create(r.Context(), service.CreateAccountDTO{
		UserID:         session.UserID,
		Name:           payload.Name,
		Type:           payload.Type,
		Currency:       payload.Currency,
		OpeningBalance: payload.OpeningBalance,
		OpeningDate:    payload.OpeningDate,
	})
	if err != nil {
		logger.Error("account_create_failed", "user_id", session.UserID, "error", err)
		c.renderAccountsModal(w, r, session.UserID, session.CSRFToken, buildCreateAccountFormState(r, err.Error()))
		return
	}

	logger.Debug("account_create_succeeded", "user_id", session.UserID, "account_name", strings.TrimSpace(r.FormValue("name")))
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) EditForm(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_edit_form_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid account id", http.StatusBadRequest)
		return
	}

	account, err := c.accountService.GetByID(r.Context(), session.UserID, accountID)
	if err != nil {
		logger.Error("account_edit_form_load_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "failed to load account details", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountEditForm(templates.NewEditAccountFormState(account, ""), session.CSRFToken))
}

func (c *Controller) ShowItem(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_show_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid account id", http.StatusBadRequest)
		return
	}

	account, err := c.accountService.GetByID(r.Context(), session.UserID, accountID)
	if err != nil {
		logger.Error("account_show_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "failed to load account", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountItem(account))
}

func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_update_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		logger.Warn("account_update_error", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "invalid account id", http.StatusBadRequest)
		return
	}

	payload, status, err := parseAccountPayload(r)
	if err != nil {
		logger.Warn("account_update_error", "user_id", session.UserID, "account_id", accountID, "error", err)
		render.Templ(w, r, status, templates.AccountEditForm(buildEditAccountFormState(accountID, r, err.Error()), session.CSRFToken))
		return
	}

	updatedAccount, err := c.accountService.Update(r.Context(), service.UpdateAccountDTO{
		UserID:         session.UserID,
		AccountID:      accountID,
		Name:           payload.Name,
		Type:           payload.Type,
		Currency:       payload.Currency,
		OpeningBalance: payload.OpeningBalance,
		OpeningDate:    payload.OpeningDate,
	})
	if err != nil {
		logger.Error("account_update_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		render.Templ(w, r, http.StatusInternalServerError, templates.AccountEditForm(buildEditAccountFormState(accountID, r, err.Error()), session.CSRFToken))
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountItem(updatedAccount))
}

func (c *Controller) AccountModal(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_modal_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountModalDialog(templates.NewCreateAccountFormState(), session.CSRFToken))
}

func (c *Controller) renderAccountsModal(w http.ResponseWriter, r *http.Request, userID int64, csrfToken string, form templates.AccountFormState) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("render_accounts_modal_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if csrfToken == "" {
		csrfToken = session.CSRFToken
	}

	render.Templ(w, r, http.StatusOK, templates.AccountModalDialog(templates.NewCreateAccountFormState(), session.CSRFToken))
}

func (c *Controller) renderAccountsPanel(w http.ResponseWriter, r *http.Request, userID int64, csrfToken string, form templates.AccountFormState) {
	accounts, err := c.accountService.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountPanels(csrfToken, accounts, form))
}

func parseAccountPayload(r *http.Request) (accountPayload, int, error) {
	if err := r.ParseForm(); err != nil {
		return accountPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	openingBalance := normalizeAmount(r.FormValue("opening_balance"))
	if openingBalance == "" {
		return accountPayload{}, http.StatusBadRequest, errors.New("opening balance must be a valid number")
	}

	openingBalanceFloat, err := strconv.ParseFloat(openingBalance, 64)
	if err != nil {
		return accountPayload{}, http.StatusBadRequest, errors.New("opening balance must be a valid number")
	}

	var openingDate *time.Time
	if value := strings.TrimSpace(r.FormValue("opening_date")); value != "" {
		parsedDate, err := time.Parse("2006-01-02", value)
		if err != nil {
			return accountPayload{}, http.StatusBadRequest, errors.New("opening date must be in the format YYYY-MM-DD")
		}
		openingDate = &parsedDate
	}

	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	if currency == "" {
		return accountPayload{}, http.StatusBadRequest, errors.New("currency is required")
	}

	return accountPayload{
		OpeningBalance: openingBalanceFloat,
		OpeningDate:    openingDate,
		Name:           strings.TrimSpace(r.FormValue("name")),
		Type:           strings.TrimSpace(r.FormValue("type")),
		Currency:       currency,
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

func buildCreateAccountFormState(r *http.Request, errorMessage string) templates.AccountFormState {
	return templates.NewCreateAccountFormStateFromValues(
		strings.TrimSpace(r.FormValue("name")),
		strings.TrimSpace(r.FormValue("type")),
		strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
		strings.TrimSpace(r.FormValue("opening_balance")),
		strings.TrimSpace(r.FormValue("opening_date")),
		errorMessage,
	)
}

func buildEditAccountFormState(accountID int64, r *http.Request, errorMessage string) templates.AccountFormState {
	return templates.NewEditAccountFormStateFromValues(
		accountID,
		strings.TrimSpace(r.FormValue("name")),
		strings.TrimSpace(r.FormValue("type")),
		strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
		strings.TrimSpace(r.FormValue("opening_balance")),
		strings.TrimSpace(r.FormValue("opening_date")),
		errorMessage,
	)
}
