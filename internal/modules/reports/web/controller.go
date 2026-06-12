package web

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"strconv"
	"time"
)

type ReportsController struct {
	reportSvc service.ReportsService
}

func NewReportsController(serv service.ReportsService) *ReportsController {
	return &ReportsController{reportSvc: serv}
}

func (c *ReportsController) Page(w http.ResponseWriter, r *http.Request) {
	render.Templ(w, r, http.StatusOK, templates.ReportsPage(csrfToken(r)))
}

func (c *ReportsController) Spending(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	from, to := parsePeriod(r)

	data, err := c.reportSvc.SpendingByCategory(r.Context(), session.UserID, from, to)
	if err != nil {
		logger.Error("reports_spending_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao carregar dados", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.SpendingFragment(data, from, to))
}

func (c *ReportsController) Comparison(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	from, to := parsePeriod(r)

	data, err := c.reportSvc.MonthlyComparison(r.Context(), session.UserID, from, to)
	if err != nil {
		logger.Error("reports_comparison_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao carregar dados", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.ComparisonFragment(data, from, to))
}

func (c *ReportsController) Balance(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	from, to := parsePeriod(r)

	data, err := c.reportSvc.BalancedHistory(r.Context(), session.UserID, from, to)
	if err != nil {
		logger.Error("reports_balance_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao carregar dados", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.BalanceFragment(data, from, to))
}

func (c *ReportsController) Transactions(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	from, to := parsePeriod(r)

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	const pageSize = 20
	offset := int32((page - 1) * pageSize)

	filter := service.TransactionFilter{
		FromDate: &from,
		ToDate:   &to,
		Limit:    pageSize,
		Offset:   offset,
	}

	if v := r.URL.Query().Get("account_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.AccountID = &id
		}
	}
	if v := r.URL.Query().Get("category_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.CategoryID = &id
		}
	}
	if v := r.URL.Query().Get("direction"); v == "debit" || v == "credit" {
		filter.Direction = &v
	}

	data, err := c.reportSvc.ListFiltered(r.Context(), session.UserID, filter)
	if err != nil {
		logger.Error("reports_transactions_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao carregar dados", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.TransactionsFragment(data, page, pageSize, from, to))
}

func parsePeriod(r *http.Request) (from, to time.Time) {
	now := time.Now().UTC()
	from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to = now

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	return
}

func csrfToken(r *http.Request) string {
	if session, ok := middleware.SessionFromContext(r.Context()); ok {
		return session.CSRFToken
	}
	return ""
}
