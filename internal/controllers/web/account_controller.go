package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	service "finops/internal/services"
	"finops/internal/web/middleware"
)

type AccountController struct {
	accountService service.AccountService
}

func NewAccountController(accountService service.AccountService) *AccountController {
	return &AccountController{
		accountService: accountService,
	}
}

func (c *AccountController) Create(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	openingBalance := normalizeAmount(r.FormValue("opening_balance"))
	if openingBalance == "" {
		http.Error(w, "opening balance is required", http.StatusBadRequest)
		return
	}

	if _, err := strconv.ParseFloat(openingBalance, 64); err != nil {
		http.Error(w, "opening balance must be numeric", http.StatusBadRequest)
		return
	}

	var openingDate *time.Time
	if value := strings.TrimSpace(r.FormValue("opening_date")); value != "" {
		parsedDate, err := time.Parse("2006-01-02", value)
		if err != nil {
			http.Error(w, "opening date is invalid", http.StatusBadRequest)
			return
		}
		openingDate = &parsedDate
	}

	_, err := c.accountService.Create(r.Context(), service.CreateAccountDTO{
		UserID:         session.UserID,
		Name:           r.FormValue("name"),
		Type:           r.FormValue("type"),
		Currency:       r.FormValue("currency"),
		OpeningBalance: openingBalance,
		OpeningDate:    openingDate,
	})
	if err != nil {
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
