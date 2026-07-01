package accounts

import (
	"context"
	"errors"
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"strconv"
	"strings"
)

type Controller struct {
	accountService service.AccountService
}

type accountPayload struct {
	Name           string
	Type           string
	Currency       string
	CurrentBalance float64
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
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	payload, status, err := parseAccountPayload(r)
	if err != nil {
		logger.Warn("account_create_invalid_payload", "user_id", session.UserID, "status", status, "error", err)
		c.renderAccountsModal(w, r, session.CSRFToken, buildCreateAccountFormState(r, err.Error()))
		return
	}

	_, err = c.accountService.Create(r.Context(), service.CreateAccountDTO{
		UserID:         session.UserID,
		Name:           payload.Name,
		Type:           payload.Type,
		Currency:       payload.Currency,
		CurrentBalance: payload.CurrentBalance,
	})
	if err != nil {
		logger.Error("account_create_failed", "user_id", session.UserID, "error", err)
		c.renderAccountsModal(w, r, session.CSRFToken, buildCreateAccountFormState(r, err.Error()))
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

	accountDTO, err := c.accountDTOFromSummary(r.Context(), session.UserID, accountID)
	if err != nil {
		logger.Error("account_edit_form_summary_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "failed to load account summary", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountModalDialog(templates.NewEditAccountFormState(account, accountDTO.CurrentBalance, ""), session.CSRFToken))
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

	accountDTO, err := c.accountDTOFromSummary(r.Context(), session.UserID, accountID)
	if err != nil {
		logger.Error("account_show_summary_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "failed to load account summary", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.AccountItem(accountDTO))
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
		render.Templ(w, r, status, templates.AccountModalDialog(buildEditAccountFormState(accountID, r, err.Error()), session.CSRFToken))
		return
	}

	_, err = c.accountService.Update(r.Context(), service.UpdateAccountDTO{
		UserID:         session.UserID,
		AccountID:      accountID,
		Name:           payload.Name,
		Type:           payload.Type,
		Currency:       payload.Currency,
		CurrentBalance: payload.CurrentBalance,
	})
	if err != nil {
		logger.Error("account_update_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		render.Templ(w, r, http.StatusInternalServerError, templates.AccountModalDialog(buildEditAccountFormState(accountID, r, err.Error()), session.CSRFToken))
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) DeleteModal(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	account, err := c.accountService.GetByID(r.Context(), session.UserID, accountID)
	if err != nil {
		logger.Warn("account_delete_modal_not_found", "user_id", session.UserID, "id", accountID, "error", err)
		http.Error(w, "conta não encontrada", http.StatusNotFound)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.AccountDeleteModalDialog(account.ID, account.Name, session.CSRFToken))
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := c.accountService.Delete(r.Context(), session.UserID, accountID); err != nil {
		logger.Error("account_delete_failed", "user_id", session.UserID, "account_id", accountID, "error", err)
		http.Error(w, "erro ao excluir conta", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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

func (c *Controller) renderAccountsModal(w http.ResponseWriter, r *http.Request, csrfToken string, form templates.AccountFormState) {

	if csrfToken == "" {
		logger := observability.Logger(r.Context())
		session, ok := middleware.SessionFromContext(r.Context())
		if !ok {
			logger.Warn("render_accounts_modal_unauthorized")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		csrfToken = session.CSRFToken
	}

	render.Templ(w, r, http.StatusOK, templates.AccountModalDialog(templates.NewCreateAccountFormStateFromValues(form.Name, form.Type, form.Currency, form.CurrentBalance, form.ErrorMessage), csrfToken))
}

func (c *Controller) accountDTOFromSummary(ctx context.Context, userID, accountID int64) (templates.AccountItemDTO, error) {
	summaries, err := c.accountService.ListSummariesByUser(ctx, userID)
	if err != nil {
		return templates.AccountItemDTO{}, err
	}

	for _, summary := range summaries {
		if summary.ID == accountID {
			return templates.AccountItemDTO{
				ID:             summary.ID,
				Name:           summary.Name,
				Type:           summary.Type,
				Currency:       summary.Currency,
				CurrentBalance: summary.CurrentBalance,
			}, nil
		}
	}

	return templates.AccountItemDTO{}, errors.New("account summary not found")
}

func parseAccountPayload(r *http.Request) (accountPayload, int, error) {
	if err := r.ParseForm(); err != nil {
		return accountPayload{}, http.StatusBadRequest, errors.New("invalid form data")
	}

	currentBalance := normalizeAmount(r.FormValue("current_balance"))
	if currentBalance == "" {
		return accountPayload{}, http.StatusBadRequest, errors.New("current balance must be a valid number")
	}

	currentBalanceFloat, err := strconv.ParseFloat(currentBalance, 64)
	if err != nil {
		return accountPayload{}, http.StatusBadRequest, errors.New("current balance must be a valid number")
	}

	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	if currency == "" {
		return accountPayload{}, http.StatusBadRequest, errors.New("currency is required")
	}

	return accountPayload{
		CurrentBalance: currentBalanceFloat,
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
		strings.TrimSpace(r.FormValue("current_balance")),
		errorMessage,
	)
}

func buildEditAccountFormState(accountID int64, r *http.Request, errorMessage string) templates.AccountFormState {
	return templates.NewEditAccountFormStateFromValues(
		accountID,
		strings.TrimSpace(r.FormValue("name")),
		strings.TrimSpace(r.FormValue("type")),
		strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
		strings.TrimSpace(r.FormValue("current_balance")),
		errorMessage,
	)
}
