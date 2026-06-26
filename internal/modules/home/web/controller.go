package home

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"time"
)

type Controller struct {
	accountService   service.AccountService
	workspaceService service.WorkspaceService
	reportService    service.ReportsService
	importService    service.ImportService
	tourService      service.TourService
	categoryService  service.CategoryService
}

func NewController(
	accountSvc service.AccountService,
	workspaceSvc service.WorkspaceService,
	reportSvc service.ReportsService,
	importSvc service.ImportService,
	tourSvc service.TourService,
	categorySvc service.CategoryService,
) *Controller {
	return &Controller{
		accountService:   accountSvc,
		workspaceService: workspaceSvc,
		reportService:    reportSvc,
		importService:    importSvc,
		tourService:      tourSvc,
		categoryService:  categorySvc,
	}
}

func (c *Controller) Home(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	exists, err := c.workspaceService.ExistsForUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("home_workspace_check_failed", "error", err)
		http.Error(w, "failed to load workspace", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}

	if r.URL.Query().Get("tour") == "" {
		done, err := c.tourService.HasDoneTour(r.Context(), session.UserID)
		if err == nil && !done {
			http.Redirect(w, r, "/tour", http.StatusSeeOther)
			return
		}
	}

	render.Templ(w, r, http.StatusOK, templates.HomePage(session.Email, session.CSRFToken))
}

func (c *Controller) Dashboard(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	period, from, to := parsePeriod(r)

	accounts, err := c.accountService.ListSummariesByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("dashboard_accounts_failed", "error", err)
	}
	var totalBalance float64
	for _, a := range accounts {
		totalBalance += a.CurrentBalance
	}

	spending, _ := c.reportService.SpendingByCategory(r.Context(), session.UserID, from, to)
	cashflow, _ := c.reportService.MonthlyComparison(r.Context(), session.UserID, from, to)
	balanceHist, _ := c.reportService.BalancedHistory(r.Context(), session.UserID, from, to)

	prevFrom, prevTo := service.PreviousPeriod(from, to)
	prevSpending, _ := c.reportService.SpendingByCategory(r.Context(), session.UserID, prevFrom, prevTo)
	prevCashflow, _ := c.reportService.MonthlyComparison(r.Context(), session.UserID, prevFrom, prevTo)

	data := service.BuildDashboard(totalBalance, spending, cashflow, balanceHist, prevSpending, prevCashflow)

	uncategorized, _ := c.categoryService.GetUncategorized(r.Context(), session.UserID)
	pending := int(uncategorized)
	last, err := c.importService.LastImport(r.Context(), session.UserID)
	if err == nil && last.Before(time.Now().AddDate(0, 0, -3)) {
		pending++
	}

	render.Templ(w, r, http.StatusOK, templates.DashboardPartial(data, period, from, to, pending, session.CSRFToken))
}

func parsePeriod(r *http.Request) (period string, from, to time.Time) {
	now := time.Now().UTC()
	period = r.URL.Query().Get("period")
	if period == "" {
		period = "this_month"
	}
	switch period {
	case "last_30":
		from = now.AddDate(0, 0, -30)
		to = now
	case "last_90":
		from = now.AddDate(0, 0, -90)
		to = now
	case "this_year":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		to = now
	case "custom":
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
		if from.IsZero() {
			from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
		if to.IsZero() {
			to = now
		}
	default: // this_month
		period = "this_month"
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = now
	}
	return
}
