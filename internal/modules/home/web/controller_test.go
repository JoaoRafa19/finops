package home

import (
	"context"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"finops/internal/models"
	service "finops/internal/services"
	"finops/internal/web/templates"
)

func TestParsePeriodDefaultsToLast30(t *testing.T) {
	req := httptest.NewRequest("GET", "/dashboard", nil)
	period, from, to := parsePeriod(req)

	if period != "last_30" {
		t.Fatalf("default period = %q, want last_30", period)
	}
	if days := to.Sub(from).Hours() / 24; days < 29 || days > 31 {
		t.Fatalf("default range = %.1f days, want ~30", days)
	}
}

func TestParsePeriodExplicitValues(t *testing.T) {
	cases := map[string]string{
		"this_month": "this_month",
		"last_30":    "last_30",
		"last_90":    "last_90",
		"this_year":  "this_year",
		"invalido":   "this_month", // fallback de valor desconhecido
	}
	for input, want := range cases {
		req := httptest.NewRequest("GET", "/dashboard?period="+input, nil)
		period, from, to := parsePeriod(req)
		if period != want {
			t.Errorf("period %q => %q, want %q", input, period, want)
		}
		if from.IsZero() || to.IsZero() || from.After(to) {
			t.Errorf("period %q => invalid range [%v, %v]", input, from, to)
		}
	}
}

func renderHome(t *testing.T) string {
	t.Helper()
	return renderHomeTour(t, false)
}

func renderHomeTour(t *testing.T, tourActive bool) string {
	t.Helper()
	var sb strings.Builder
	if err := templates.HomePage("user@example.com", "csrf-token", tourActive, service.HomeModeAdvanced).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render home: %v", err)
	}
	return sb.String()
}

func TestHomePeriodSelectDefaultsToLast30(t *testing.T) {
	html := renderHome(t)

	if !regexp.MustCompile(`value="last_30"\s+selected`).MatchString(html) {
		t.Error("option last_30 is not selected by default")
	}
	if !strings.Contains(html, `hx-get="/dashboard?period=last_30"`) {
		t.Error("initial dashboard load does not use period=last_30")
	}
}

func TestHomeClosesCategoryModalOnSuccess(t *testing.T) {
	html := renderHome(t)

	if !strings.Contains(html, `addEventListener("category-created"`) {
		t.Error("home has no listener for category-created event (modal never closes)")
	}
	if !strings.Contains(html, `getElementById("category-modal")?.close()`) {
		t.Error("category-created listener does not close #category-modal")
	}
}

// Em modo tour o dashboard carrega com tour=1 (dados mock, banco intocado).
func TestHomeTourModeLoadsMockDashboard(t *testing.T) {
	if html := renderHomeTour(t, true); !strings.Contains(html, `hx-get="/dashboard?period=last_30&tour=1"`) {
		t.Error("tour mode should load dashboard with tour=1")
	}
	if html := renderHome(t); strings.Contains(html, `hx-get="/dashboard?period=last_30&tour=1"`) {
		t.Error("normal mode must not load dashboard with tour=1")
	}
}

// Dashboard em modo tour responde com dados mock sem consultar nenhum serviço
// (controller com serviços nil: qualquer acesso ao banco causaria panic).
func TestDashboardTourModeUsesMockWithoutServices(t *testing.T) {
	c := NewController(nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/dashboard?tour=1", nil)
	req = req.WithContext(context.WithValue(req.Context(), models.SessionCtxKey, models.Session{
		UserID: 1, Email: "user@example.com", CSRFToken: "csrf-token",
	}))
	rec := httptest.NewRecorder()
	c.Dashboard(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Alimentação") {
		t.Error("tour dashboard should render mock data")
	}
}

// Garante que todo passo do tour na rota "/" aponta para um elemento
// que realmente existe na home renderizada (regressão de UI vs tour).
func TestTourStepsTargetExistingHomeElements(t *testing.T) {
	html := renderHome(t)

	stepRe := regexp.MustCompile(`path: "/", element: "#([a-zA-Z0-9_-]+)"`)
	matches := stepRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		t.Fatal("no tour steps found for path '/' in rendered home")
	}
	for _, m := range matches {
		id := m[1]
		if !strings.Contains(html, `id="`+id+`"`) {
			t.Errorf("tour step targets #%s but home has no element with that id", id)
		}
	}
}
