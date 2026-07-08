package projections

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"finops/internal/models"
	service "finops/internal/services"
	"finops/internal/store"
)

type projectionServiceStub struct {
	created *service.Commitment
}

func (s *projectionServiceStub) Forecast(_ context.Context, _ int64, from, to time.Time) (service.ProjectionResult, error) {
	return service.ProjectionResult{
		Months: []service.MonthProjection{
			{Month: from, Income: 5885, FixedOut: 4425.43, VariableOut: 800, Net: 659.57, Cumulative: 1459.57, CardTotals: map[int64]float64{1: 2205.23}},
			{Month: to, Income: 5885, Net: -100, Cumulative: 1359.57, CardTotals: map[int64]float64{}},
		},
		CardNames:  map[int64]string{1: "BB Ourocard"},
		Summary:    service.ProjectionSummary{AvgNet: 279.79, WorstNet: -100, WorstMonth: to, BestNet: 659.57, BestMonth: from, FinalCash: 1359.57, MonthsNetNegative: 1},
		Milestones: []service.Milestone{{Month: from, Label: "Última parcela: KaBuM"}},
	}, nil
}
func (s *projectionServiceStub) Simulate(_ context.Context, _ int64, _, _ time.Time) (service.FinancingSimulation, error) {
	return service.FinancingSimulation{AmountFinanced: 279200, PriceInstallment: 2456, PriceShare: 1228, Capacity: 2678.90, FitsPrice: true, FitsSac: true, Verdict: "CABE — cabe na capacidade projetada."}, nil
}
func (s *projectionServiceStub) SetVariableOverride(_ context.Context, _ int64, _ time.Time, _ float64) error {
	return nil
}
func (s *projectionServiceStub) ClearVariableOverride(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (s *projectionServiceStub) ListCommitments(_ context.Context, _ int64) ([]service.Commitment, error) {
	return []service.Commitment{{ID: 1, Name: "KaBuM", Kind: service.CommitmentInstallment, MonthlyValue: 718.38, StartMonth: time.Now()}}, nil
}
func (s *projectionServiceStub) GetCommitment(_ context.Context, _, id int64) (service.Commitment, error) {
	return service.Commitment{ID: id, Name: "KaBuM", Kind: service.CommitmentInstallment, MonthlyValue: 718.38, StartMonth: time.Now()}, nil
}
func (s *projectionServiceStub) CreateCommitment(_ context.Context, _ int64, c service.Commitment) (service.Commitment, error) {
	if c.MonthlyValue <= 0 {
		return service.Commitment{}, errors.New("valor mensal deve ser maior que zero")
	}
	s.created = &c
	return c, nil
}
func (s *projectionServiceStub) UpdateCommitment(_ context.Context, _ int64, _ service.Commitment) error {
	return nil
}
func (s *projectionServiceStub) DeleteCommitment(_ context.Context, _, _ int64) error { return nil }
func (s *projectionServiceStub) GetSettings(_ context.Context, _ int64) (service.ProjectionSettings, error) {
	return service.ProjectionSettings{MonthlyIncome: 5885}, nil
}
func (s *projectionServiceStub) SaveSettings(_ context.Context, _ int64, _ service.ProjectionSettings) error {
	return nil
}

type accountServiceStub struct{}

func (accountServiceStub) ListByUser(_ context.Context, _ int64) ([]store.Account, error) {
	return []store.Account{{ID: 1, Name: "BB Ourocard"}}, nil
}
func (accountServiceStub) ListSummariesByUser(_ context.Context, _ int64) ([]service.AccountSummary, error) {
	return nil, nil
}
func (accountServiceStub) GetByID(_ context.Context, _, _ int64) (store.Account, error) {
	return store.Account{}, nil
}
func (accountServiceStub) Create(_ context.Context, _ service.CreateAccountDTO) (store.Account, error) {
	return store.Account{}, nil
}
func (accountServiceStub) Update(_ context.Context, _ service.UpdateAccountDTO) (store.Account, error) {
	return store.Account{}, nil
}
func (accountServiceStub) Delete(_ context.Context, _, _ int64) error { return nil }

func doRequest(t *testing.T, handler http.HandlerFunc, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	body := ""
	if form != nil {
		body = form.Encode()
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req = req.WithContext(context.WithValue(req.Context(), models.SessionCtxKey, models.Session{
		UserID:    1,
		Email:     "user@example.com",
		CSRFToken: "csrf-token",
	}))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestForecastFragmentRendersTable(t *testing.T) {
	c := NewProjectionsController(&projectionServiceStub{}, accountServiceStub{})
	rec := doRequest(t, c.Forecast, "GET", "/projections/forecast", nil)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Fluxo de caixa projetado", "BB Ourocard", "/projections/variable"} {
		if !strings.Contains(body, want) {
			t.Errorf("forecast fragment should contain %q", want)
		}
	}
}

func TestSummaryFragmentRendersKPIsAndMilestones(t *testing.T) {
	c := NewProjectionsController(&projectionServiceStub{}, accountServiceStub{})
	rec := doRequest(t, c.Summary, "GET", "/projections/summary", nil)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Vale (menor caixa)", "Última parcela: KaBuM", "Caixa acumulado"} {
		if !strings.Contains(body, want) {
			t.Errorf("summary fragment should contain %q", want)
		}
	}
}

func TestSimulatorFragmentRendersVerdict(t *testing.T) {
	c := NewProjectionsController(&projectionServiceStub{}, accountServiceStub{})
	rec := doRequest(t, c.Simulator, "GET", "/projections/simulator", nil)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Simulador de financiamento", "CABE", "Saldo a financiar"} {
		if !strings.Contains(body, want) {
			t.Errorf("simulator fragment should contain %q", want)
		}
	}
}

func TestSetVariableOverrideTriggersRefresh(t *testing.T) {
	c := NewProjectionsController(&projectionServiceStub{}, accountServiceStub{})
	rec := doRequest(t, c.SetVariableOverride, "POST", "/projections/variable", url.Values{
		"month": {"2026-08"},
		"value": {"500"},
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Trigger"); got != "projection-changed" {
		t.Errorf("HX-Trigger = %q, want projection-changed", got)
	}
	if !strings.Contains(rec.Body.String(), "Fluxo de caixa projetado") {
		t.Error("override response should re-render the forecast fragment")
	}
}

func TestCreateCommitmentSuccessTriggersRefresh(t *testing.T) {
	stub := &projectionServiceStub{}
	c := NewProjectionsController(stub, accountServiceStub{})
	rec := doRequest(t, c.CreateCommitment, "POST", "/projections/commitments", url.Values{
		"name":          {"Netflix"},
		"kind":          {"subscription"},
		"monthly_value": {"71,83"},
		"start_month":   {"2026-07"},
	})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Trigger"); got != "projection-changed" {
		t.Errorf("HX-Trigger = %q, want projection-changed", got)
	}
	if stub.created == nil {
		t.Fatal("commitment was not created")
	}
	if stub.created.MonthlyValue != 71.83 {
		t.Errorf("monthly value = %v, want 71.83 (vírgula decimal)", stub.created.MonthlyValue)
	}
	if stub.created.EndMonth != nil {
		t.Errorf("end month should be nil for open-ended subscription")
	}
}

func TestCreateCommitmentInvalidValueShowsError(t *testing.T) {
	c := NewProjectionsController(&projectionServiceStub{}, accountServiceStub{})
	rec := doRequest(t, c.CreateCommitment, "POST", "/projections/commitments", url.Values{
		"name":          {"Netflix"},
		"kind":          {"subscription"},
		"monthly_value": {"abc"},
		"start_month":   {"2026-07"},
	})

	if got := rec.Header().Get("HX-Trigger"); got != "" {
		t.Errorf("error response must not trigger refresh, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "Dados inválidos") {
		t.Error("error response should show validation message")
	}
}

func TestUpdateSettingsTriggersRefresh(t *testing.T) {
	c := NewProjectionsController(&projectionServiceStub{}, accountServiceStub{})
	rec := doRequest(t, c.UpdateSettings, "POST", "/projections/settings", url.Values{
		"monthly_income":   {"5.885,00"},
		"variable_expense": {"800"},
		"opening_balance":  {"800"},
	})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Trigger"); got != "projection-changed" {
		t.Errorf("HX-Trigger = %q, want projection-changed", got)
	}
}
