package web

import (
	"errors"
	"finops/internal/observability"
	"net/http"
	"strconv"
	"strings"
	"time"

	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/templates"
)

type AccountController struct {
	accountService service.AccountService
}

type accountPayload struct {
	Name           string
	Type           string
	Currency       string
	OpeningBalance float64
	OpeningDate    *time.Time
}

func NewAccountController(accountService service.AccountService) *AccountController {
	return &AccountController{
		accountService: accountService,
	}
}

func (c *AccountController) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("create_transaction_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Warn("create_transaction_invalid_form", "user_id", session.UserID, "error", err)
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	//amount := r.FormValue("category")

}

func parseAccountPayload(r *http.Request) (accountPayload, int, error) {
	if err := r.ParseForm(); err != nil {
		return accountPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	openingBalance := normalizeAmount(r.FormValue("opening_balance"))
	if openingBalance == "" {
		return accountPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	openingBalanceFloat, err := strconv.ParseFloat(openingBalance, 64)
	if err != nil {
		return accountPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	var openingDate *time.Time

	if value := strings.TrimSpace(r.FormValue("opening_date")); value != "" {
		parsedDate, err := time.Parse("2006-01-02", value)
		if err != nil {
			return accountPayload{}, http.StatusBadRequest, errors.New("invalid form data")
		}
		openingDate = &parsedDate
	}

	return accountPayload{
		OpeningBalance: openingBalanceFloat,
		OpeningDate:    openingDate,
		Name:           r.FormValue("name"),
		Type:           r.FormValue("type"),
		Currency:       r.FormValue("currency"),
	}, http.StatusOK, nil
}

func (c *AccountController) Create(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		logger.Warn("account_create_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Warn("account_create_invalid_form", "user_id", session.UserID, "error", err)
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	openingBalance := normalizeAmount(r.FormValue("opening_balance"))
	if openingBalance == "" {
		logger.Warn("account_create_missing_opening_balance", "user_id", session.UserID)
		http.Error(w, "opening balance is required", http.StatusBadRequest)
		return
	}
	var openingBalanceFloat float64

	openingBalanceFloat, err := strconv.ParseFloat(openingBalance, 32)
	if err != nil {
		logger.Warn("account_create_invalid_opening_balance", "user_id", session.UserID, "opening_balance", openingBalance)
		http.Error(w, "opening balance must be numeric", http.StatusBadRequest)
		return
	}

	var openingDate *time.Time
	if value := strings.TrimSpace(r.FormValue("opening_date")); value != "" {
		parsedDate, err := time.Parse("2006-01-02", value)
		if err != nil {
			logger.Warn("account_create_invalid_opening_date", "user_id", session.UserID, "opening_date", value)
			http.Error(w, "opening date is invalid", http.StatusBadRequest)
			return
		}
		openingDate = &parsedDate
	}

	_, err = c.accountService.Create(r.Context(), service.CreateAccountDTO{
		UserID:         session.UserID,
		Name:           r.FormValue("name"),
		Type:           r.FormValue("type"),
		Currency:       r.FormValue("currency"),
		OpeningBalance: openingBalanceFloat,
		OpeningDate:    openingDate,
	})
	if err != nil {
		logger.Error("account_create_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	logger.Debug("account_create_succeeded", "user_id", session.UserID, "account_name", strings.TrimSpace(r.FormValue("name")))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *AccountController) EditForm(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "failed to load account", http.StatusInternalServerError)
		return
	}

	renderTempl(w, r, http.StatusOK, templates.AccountEditForm(&account, session.CSRFToken))
}

func (c *AccountController) Update(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())

	if !ok {
		logger.Warn("account_update_unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		logger.Warn("account_update_error", "user_id", session.UserID, "account_id", accountId, "error", err)
		http.Error(w, "invalid account id", http.StatusBadRequest)
		return
	}

	payload, status, err := parseAccountPayload(r)

	if err != nil {
		logger.Warn("account_update_error", "user_id", session.UserID, "account_id", accountId, "error", err)
		http.Error(w, err.Error(), status)
		return
	}

	_, err = c.accountService.Update(r.Context(), service.UpdateAccountDTO{
		UserID:         session.UserID,
		AccountID:      accountId,
		Name:           payload.Name,
		Type:           payload.Type,
		Currency:       payload.Currency,
		OpeningBalance: payload.OpeningBalance,
		OpeningDate:    payload.OpeningDate,
	})

	if err != nil {
		logger.Error("account_update_failed", "user_id", session.UserID, "account_id", accountId, "error", err)
		http.Error(w, "failed to update account", http.StatusInternalServerError)
		return
	}

	c.renderAccountsPannel(w, r, session.UserID, session.CSRFToken)
}

func (c *AccountController) renderAccountsPannel(w http.ResponseWriter, r *http.Request, userId int64, csrfToken string) {
	accounts, err := c.accountService.ListByUser(r.Context(), userId)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	renderTempl(w, r, http.StatusOK, templates.AccountPanels(csrfToken, accounts))
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
