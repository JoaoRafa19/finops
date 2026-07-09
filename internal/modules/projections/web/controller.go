package projections

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
)

type ProjectionsController struct {
	projectionSvc service.ProjectionService
	accountSvc    service.AccountService
}

func NewProjectionsController(projectionSvc service.ProjectionService, accountSvc service.AccountService) *ProjectionsController {
	return &ProjectionsController{projectionSvc: projectionSvc, accountSvc: accountSvc}
}

// horizon resolve a janela do horizonte a partir da query (?from=YYYY-MM),
// default: mês atual → +11 (12 meses).
func horizon(r *http.Request) (time.Time, time.Time) {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01", v); err == nil {
			from = t
		}
	}
	to := from.AddDate(0, 11, 0)
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01", v); err == nil && !t.Before(from) {
			to = t
		}
	}
	return from, to
}

func (c *ProjectionsController) Page(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.ProjectionsPage(session.CSRFToken))
}

// Forecast: demonstrativo mês a mês (Projeção) — tabela + gasto variável editável.
func (c *ProjectionsController) Forecast(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	from, to := horizon(r)
	res, err := c.projectionSvc.Forecast(r.Context(), session.UserID, from, to)
	if err != nil {
		logger.Error("projections_forecast_error", "user", session.UserID, "error", err.Error())
		http.Error(w, "failed to compute forecast", http.StatusInternalServerError)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.ForecastFragment(res, session.CSRFToken))
}

// Summary: dashboard Resumo (KPIs, gráficos, marcos).
func (c *ProjectionsController) Summary(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	from, to := horizon(r)
	res, err := c.projectionSvc.Forecast(r.Context(), session.UserID, from, to)
	if err != nil {
		logger.Error("projections_summary_error", "user", session.UserID, "error", err.Error())
		http.Error(w, "failed to compute summary", http.StatusInternalServerError)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.SummaryFragment(res))
}

// Simulator: cenário de financiamento de longo prazo (§4.5).
func (c *ProjectionsController) Simulator(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	from, to := horizon(r)
	sim, err := c.projectionSvc.Simulate(r.Context(), session.UserID, from, to)
	if err != nil {
		logger.Error("projections_simulate_error", "user", session.UserID, "error", err.Error())
		http.Error(w, "failed to simulate", http.StatusInternalServerError)
		return
	}
	settings, err := c.projectionSvc.GetSettings(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.SimulatorFragment(sim, settings, session.CSRFToken))
}

// CommitmentsFragment renderiza o painel: premissas + form de criação + lista.
func (c *ProjectionsController) CommitmentsFragment(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	c.renderPanel(w, r, session.UserID, session.CSRFToken, "")
}

func (c *ProjectionsController) renderPanel(w http.ResponseWriter, r *http.Request, userID int64, csrf, errMsg string) {
	logger := observability.Logger(r.Context())
	commitments, err := c.projectionSvc.ListCommitments(r.Context(), userID)
	if err != nil {
		logger.Error("projections_list_error", "user", userID, "error", err.Error())
		http.Error(w, "failed to load commitments", http.StatusInternalServerError)
		return
	}
	settings, err := c.projectionSvc.GetSettings(r.Context(), userID)
	if err != nil {
		logger.Error("projections_settings_error", "user", userID, "error", err.Error())
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	accounts, err := c.accountSvc.ListByUser(r.Context(), userID)
	if err != nil {
		logger.Error("projections_accounts_error", "user", userID, "error", err.Error())
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.CommitmentsPanel(commitments, settings, accounts, csrf, errMsg))
}

// parseCommitmentForm monta o Commitment a partir do form (aceita vírgula decimal
// e <input type="month">).
func parseCommitmentForm(r *http.Request) (service.Commitment, error) {
	if err := r.ParseForm(); err != nil {
		return service.Commitment{}, err
	}
	c := service.Commitment{
		Name:  strings.TrimSpace(r.FormValue("name")),
		Kind:  strings.TrimSpace(r.FormValue("kind")),
		Notes: strings.TrimSpace(r.FormValue("notes")),
	}
	var err error
	if c.MonthlyValue, err = parseMoney(r.FormValue("monthly_value")); err != nil {
		return c, err
	}
	if c.StartMonth, err = time.Parse("2006-01", r.FormValue("start_month")); err != nil {
		return c, err
	}
	if v := strings.TrimSpace(r.FormValue("end_month")); v != "" {
		end, err := time.Parse("2006-01", v)
		if err != nil {
			return c, err
		}
		c.EndMonth = &end
	}
	if v := strings.TrimSpace(r.FormValue("account_id")); v != "" && v != "0" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return c, err
		}
		c.AccountID = &id
	}
	return c, nil
}

// parseMoney aceita "1.266,13" (BR) e "1266.13": com vírgula, pontos são
// separador de milhar; sem vírgula, ponto é decimal.
func parseMoney(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	return strconv.ParseFloat(s, 64)
}

func (c *ProjectionsController) CreateCommitment(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	commitment, err := parseCommitmentForm(r)
	if err == nil {
		_, err = c.projectionSvc.CreateCommitment(r.Context(), session.UserID, commitment)
	}
	if err != nil {
		logger.Warn("projections_create_error", "user", session.UserID, "error", err.Error())
		c.renderPanel(w, r, session.UserID, session.CSRFToken, "Dados inválidos: "+err.Error())
		return
	}
	logger.Info("projections_create_success", "user", session.UserID, "name", commitment.Name)
	w.Header().Set("HX-Trigger", "projection-changed")
	c.renderPanel(w, r, session.UserID, session.CSRFToken, "")
}

func (c *ProjectionsController) EditRow(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	commitment, err := c.projectionSvc.GetCommitment(r.Context(), session.UserID, id)
	if err != nil {
		http.Error(w, "compromisso não encontrado", http.StatusNotFound)
		return
	}
	accounts, err := c.accountSvc.ListByUser(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.CommitmentEditRow(commitment, accounts, session.CSRFToken, ""))
}

func (c *ProjectionsController) Row(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	commitment, err := c.projectionSvc.GetCommitment(r.Context(), session.UserID, id)
	if err != nil {
		http.Error(w, "compromisso não encontrado", http.StatusNotFound)
		return
	}
	render.Templ(w, r, http.StatusOK, templates.CommitmentRow(commitment, session.CSRFToken))
}

func (c *ProjectionsController) UpdateCommitment(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	commitment, err := parseCommitmentForm(r)
	commitment.ID = id
	if err == nil {
		err = c.projectionSvc.UpdateCommitment(r.Context(), session.UserID, commitment)
	}
	if err != nil {
		logger.Warn("projections_update_error", "user", session.UserID, "id", id, "error", err.Error())
		accounts, accErr := c.accountSvc.ListByUser(r.Context(), session.UserID)
		if accErr != nil {
			http.Error(w, "failed to load accounts", http.StatusInternalServerError)
			return
		}
		render.Templ(w, r, http.StatusOK, templates.CommitmentEditRow(commitment, accounts, session.CSRFToken, err.Error()))
		return
	}
	updated, err := c.projectionSvc.GetCommitment(r.Context(), session.UserID, id)
	if err != nil {
		http.Error(w, "compromisso não encontrado", http.StatusNotFound)
		return
	}
	w.Header().Set("HX-Trigger", "projection-changed")
	render.Templ(w, r, http.StatusOK, templates.CommitmentRow(updated, session.CSRFToken))
}

func (c *ProjectionsController) DeleteCommitment(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := c.projectionSvc.DeleteCommitment(r.Context(), session.UserID, id); err != nil {
		logger.Warn("projections_delete_error", "user", session.UserID, "id", id, "error", err.Error())
		http.Error(w, "erro ao excluir", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "projection-changed")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{})
}

// UpdateSettings salva as premissas (renda, variável, saldo + parâmetros do simulador).
func (c *ProjectionsController) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	s, err := parseSettingsForm(r)
	if err == nil {
		err = c.projectionSvc.SaveSettings(r.Context(), session.UserID, s)
	}
	if err != nil {
		logger.Warn("projections_settings_save_error", "user", session.UserID, "error", err.Error())
		render.Templ(w, r, http.StatusOK, templates.ProjectionSettingsForm(s, session.CSRFToken, "Valores inválidos"))
		return
	}
	logger.Info("projections_settings_saved", "user", session.UserID)
	w.Header().Set("HX-Trigger", "projection-changed")
	render.Templ(w, r, http.StatusOK, templates.ProjectionSettingsForm(s, session.CSRFToken, ""))
}

func parseSettingsForm(r *http.Request) (service.ProjectionSettings, error) {
	var s service.ProjectionSettings
	fields := []struct {
		name string
		dst  *float64
	}{
		{"monthly_income", &s.MonthlyIncome},
		{"variable_expense", &s.VariableExpense},
		{"opening_balance", &s.OpeningBalance},
		{"property_value", &s.PropertyValue},
		{"down_payment_monthly", &s.DownPaymentMonthly},
	}
	for _, f := range fields {
		v, err := parseMoney(r.FormValue(f.name))
		if err != nil {
			return s, err
		}
		*f.dst = v
	}
	s.DownPaymentMonths = parseIntDefault(r.FormValue("down_payment_months"), 0)
	s.FinancingTermYears = parseIntDefault(r.FormValue("financing_term_years"), 0)
	// Taxa anual em % (ex.: "10,5" → 0.105).
	if rate, err := parseMoney(r.FormValue("financing_annual_rate")); err == nil {
		s.FinancingAnnualRate = rate / 100
	}
	// Percentual da sua parte (ex.: "50" → 0.5); default 100%.
	if share, err := parseMoney(r.FormValue("share_pct")); err == nil && share > 0 {
		s.SharePct = share / 100
	} else {
		s.SharePct = 1
	}
	return s, nil
}

func parseIntDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

// SetVariableOverride grava um gasto variável manual para um mês (item 11).
func (c *ProjectionsController) SetVariableOverride(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	month, err := time.Parse("2006-01", r.FormValue("month"))
	if err != nil {
		http.Error(w, "mês inválido", http.StatusBadRequest)
		return
	}
	raw := strings.TrimSpace(r.FormValue("value"))
	if raw == "" {
		// Campo esvaziado = volta ao gasto variável padrão.
		if err := c.projectionSvc.ClearVariableOverride(r.Context(), session.UserID, month); err != nil {
			logger.Warn("projections_var_clear_error", "user", session.UserID, "error", err.Error())
			http.Error(w, "erro ao limpar", http.StatusInternalServerError)
			return
		}
	} else {
		value, err := parseMoney(raw)
		if err != nil {
			http.Error(w, "valor inválido", http.StatusBadRequest)
			return
		}
		if err := c.projectionSvc.SetVariableOverride(r.Context(), session.UserID, month, value); err != nil {
			logger.Warn("projections_var_set_error", "user", session.UserID, "error", err.Error())
			http.Error(w, "erro ao salvar", http.StatusInternalServerError)
			return
		}
	}
	// Re-renderiza o fragment inteiro (dispara recálculo de todos os meses).
	from, to := horizon(r)
	res, err := c.projectionSvc.Forecast(r.Context(), session.UserID, from, to)
	if err != nil {
		http.Error(w, "failed to recompute", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "projection-changed")
	render.Templ(w, r, http.StatusOK, templates.ForecastFragment(res, session.CSRFToken))
}
