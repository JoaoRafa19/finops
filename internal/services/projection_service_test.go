package service

import (
	"math"
	"strings"
	"testing"
	"time"
)

func month(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func mp(t time.Time) *time.Time { return &t }

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) >= 0.005 {
		t.Errorf("%s: got %.2f, want %.2f", label, got, want)
	}
}

// Fixture: réplica da planilha Projecao_Financeira_Joao.xlsx (Jul/26–Jun/27).
// Os asserts vêm da própria planilha (spec §4.4/4.6) — se o motor divergir, quebrou.
func planilhaFixture() (ProjectionSettings, []Commitment) {
	settings := ProjectionSettings{
		MonthlyIncome:   5885,
		VariableExpense: 800,
		OpeningBalance:  800,
		// Simulador (Premissas §4.2)
		PropertyValue:       344000,
		DownPaymentMonthly:  1800,
		DownPaymentMonths:   36,
		FinancingAnnualRate: 0.105,
		FinancingTermYears:  30,
		SharePct:            0.5,
	}

	bb, itau := int64(1), int64(2)
	cs := []Commitment{
		{Name: "Empréstimo mãe Ago", Kind: CommitmentIncome, MonthlyValue: 1075, StartMonth: month(2026, 8), EndMonth: mp(month(2026, 8))},
		{Name: "Empréstimo mãe Set", Kind: CommitmentIncome, MonthlyValue: 1060, StartMonth: month(2026, 9), EndMonth: mp(month(2026, 9))},
		{Name: "Empréstimo mãe Out", Kind: CommitmentIncome, MonthlyValue: 1045, StartMonth: month(2026, 10), EndMonth: mp(month(2026, 10))},
		{Name: "Empréstimo mãe Nov", Kind: CommitmentIncome, MonthlyValue: 1030, StartMonth: month(2026, 11), EndMonth: mp(month(2026, 11))},
		{Name: "Empréstimo mãe Dez", Kind: CommitmentIncome, MonthlyValue: 1015, StartMonth: month(2026, 12), EndMonth: mp(month(2026, 12))},
		{Name: "PLR", Kind: CommitmentIncome, MonthlyValue: 10000, StartMonth: month(2027, 1), EndMonth: mp(month(2027, 1))},

		{Name: "Curso PUC", Kind: CommitmentFixed, MonthlyValue: 1266.13, StartMonth: month(2026, 8), EndMonth: mp(month(2028, 1))},
		{Name: "MacBook", Kind: CommitmentInstallment, MonthlyValue: 1195, StartMonth: month(2026, 8), EndMonth: mp(month(2026, 12))},
		{Name: "Curso de Go", Kind: CommitmentInstallment, MonthlyValue: 173.91, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 12))},
		{Name: "Entrada apartamento", Kind: CommitmentFixed, MonthlyValue: 950, StartMonth: month(2026, 7)},
		{Name: "Documentação apê", Kind: CommitmentOneOff, MonthlyValue: 8500, StartMonth: month(2026, 11), EndMonth: mp(month(2026, 11))},

		{AccountID: &bb, AccountName: "BB Ourocard", Name: "KaBuM", Kind: CommitmentInstallment, MonthlyValue: 718.38, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 8))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Empréstimo PG", Kind: CommitmentInstallment, MonthlyValue: 167.62, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 12))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Shopee", Kind: CommitmentInstallment, MonthlyValue: 242.23, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 12))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Amazon Marketplace", Kind: CommitmentInstallment, MonthlyValue: 101.59, StartMonth: month(2026, 7), EndMonth: mp(month(2027, 2))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "AliExpress", Kind: CommitmentInstallment, MonthlyValue: 147.82, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 8))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Amazon Market", Kind: CommitmentInstallment, MonthlyValue: 47.49, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 7))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "EC Belo Horizonte", Kind: CommitmentInstallment, MonthlyValue: 144.78, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 8))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Amazon série B", Kind: CommitmentInstallment, MonthlyValue: 78.08, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 10))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "CEA", Kind: CommitmentInstallment, MonthlyValue: 113.34, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 9))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Netflix STLFLIX", Kind: CommitmentInstallment, MonthlyValue: 71.83, StartMonth: month(2026, 7), EndMonth: mp(month(2027, 2))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Amazon BR", Kind: CommitmentInstallment, MonthlyValue: 64.77, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 8))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Anuidade", Kind: CommitmentInstallment, MonthlyValue: 17.50, StartMonth: month(2026, 7), EndMonth: mp(month(2027, 3))},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Conta Vivo", Kind: CommitmentSubscription, MonthlyValue: 200, StartMonth: month(2026, 7)},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Amazon Prime", Kind: CommitmentSubscription, MonthlyValue: 29.90, StartMonth: month(2026, 7)},
		{AccountID: &bb, AccountName: "BB Ourocard", Name: "Microsoft", Kind: CommitmentSubscription, MonthlyValue: 59.90, StartMonth: month(2026, 7)},

		{AccountID: &itau, AccountName: "Itaú Platinum", Name: "Carapreta", Kind: CommitmentInstallment, MonthlyValue: 742.54, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 7))},
		{AccountID: &itau, AccountName: "Itaú Platinum", Name: "Leitura Shopping", Kind: CommitmentInstallment, MonthlyValue: 140.90, StartMonth: month(2026, 7), EndMonth: mp(month(2026, 9))},
		{AccountID: &itau, AccountName: "Itaú Platinum", Name: "HBO Max", Kind: CommitmentSubscription, MonthlyValue: 44.90, StartMonth: month(2026, 7)},
		{AccountID: &itau, AccountName: "Itaú Platinum", Name: "Claude.ai", Kind: CommitmentSubscription, MonthlyValue: 121.05, StartMonth: month(2026, 7)},
		{AccountID: &itau, AccountName: "Itaú Platinum", Name: "Disney+", Kind: CommitmentSubscription, MonthlyValue: 46.90, StartMonth: month(2026, 7)},
	}
	return settings, cs
}

func TestComputeForecastPlanilha(t *testing.T) {
	settings, cs := planilhaFixture()
	res := computeForecast(settings, cs, nil, month(2026, 7), month(2027, 6))

	if len(res.Months) != 12 {
		t.Fatalf("meses: got %d, want 12", len(res.Months))
	}

	jul := res.Months[0]
	approx(t, "entradas Jul/26", jul.Income, 5885)
	approx(t, "saídas Jul/26", jul.TotalOut, 5225.43)
	approx(t, "sobra Jul/26", jul.Net, 659.57)
	approx(t, "cartão BB Jul/26", jul.CardTotals[1], 2205.23)
	approx(t, "cartão Itaú Jul/26", jul.CardTotals[2], 1096.29)

	nov := res.Months[4]
	approx(t, "sobra Nov/26", nov.Net, -7073.46)

	jan := res.Months[6]
	approx(t, "entradas Jan/27 (PLR)", jan.Income, 15885)
	approx(t, "sobra Jan/27", jan.Net, 12175.30)

	ago := res.Months[1]
	approx(t, "comprometimento Ago/26", ago.CommittedPct, 0.9909)

	approx(t, "caixa acumulado Jun/27", res.Summary.FinalCash, 22146.78)
	approx(t, "pior mês", res.Summary.WorstNet, -7073.46)
	if !res.Summary.WorstMonth.Equal(month(2026, 11)) {
		t.Errorf("pior mês: got %v, want Nov/26", res.Summary.WorstMonth)
	}
	approx(t, "melhor mês", res.Summary.BestNet, 12175.30)
	approx(t, "sobra média", res.Summary.AvgNet, 1778.90)
	if res.Summary.MonthsNetNegative != 1 {
		t.Errorf("meses sobra negativa: got %d, want 1", res.Summary.MonthsNetNegative)
	}
}

// KPIs novos da v2.0: o "vale" (menor caixa acumulado) e meses com caixa negativo.
// Na planilha, o vale é ~−R$3.062,74 em Nov/26 e há 2 meses de caixa negativo (Nov+Dez).
func TestComputeForecastLowestCashKPIs(t *testing.T) {
	settings, cs := planilhaFixture()
	res := computeForecast(settings, cs, nil, month(2026, 7), month(2027, 6))

	approx(t, "vale (menor caixa)", res.Summary.LowestCash, -3062.74)
	if !res.Summary.LowestCashMonth.Equal(month(2026, 11)) {
		t.Errorf("mês do vale: got %v, want Nov/26", res.Summary.LowestCashMonth)
	}
	if res.Summary.MonthsCashNegative != 2 {
		t.Errorf("meses caixa negativo: got %d, want 2 (Nov+Dez)", res.Summary.MonthsCashNegative)
	}
}

func TestComputeForecastVariableOverride(t *testing.T) {
	settings, cs := planilhaFixture()
	// "vou segurar em R$500 em agosto" (item 11 da Projeção).
	overrides := map[string]float64{"2026-08": 500}
	res := computeForecast(settings, cs, overrides, month(2026, 7), month(2027, 6))

	ago := res.Months[1]
	if !ago.VariableIsOverride {
		t.Error("Ago/26 deveria estar marcado como override")
	}
	approx(t, "gasto variável Ago com override", ago.VariableOut, 500)
	// sobra base de Ago era 63,47; com −300 de variável, sobe para 363,47.
	approx(t, "sobra Ago com override", ago.Net, 363.47)

	jul := res.Months[0]
	if jul.VariableIsOverride {
		t.Error("Jul/26 não tem override, não deveria estar marcado")
	}
	approx(t, "gasto variável Jul (default)", jul.VariableOut, 800)
}

func TestComputeForecastMilestones(t *testing.T) {
	settings, cs := planilhaFixture()
	res := computeForecast(settings, cs, nil, month(2026, 7), month(2027, 6))

	wants := map[string]time.Time{
		"Última parcela: KaBuM":          month(2026, 8),
		"Última parcela: MacBook":        month(2026, 12),
		"Entrada pontual: PLR":           month(2027, 1),
		"Evento único: Documentação apê": month(2026, 11),
	}
	for label, wantMonth := range wants {
		found := false
		for _, m := range res.Milestones {
			if m.Label == label && m.Month.Equal(wantMonth) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("marco %q em %v não encontrado", label, wantMonth.Format("2006-01"))
		}
	}
	for _, m := range res.Milestones {
		if strings.Contains(m.Label, "PUC") {
			t.Errorf("marco indevido fora do horizonte: %s", m.Label)
		}
	}
}

// Simulador (§4.5 / RN-07): saldo a financiar, parcela Price e veredito.
func TestComputeFinancing(t *testing.T) {
	settings, _ := planilhaFixture()

	// Saldo a financiar = 344.000 − 1.800×36 = 344.000 − 64.800 = 279.200 (spec §4.5).
	sim := computeFinancing(settings, 1778.90)
	approx(t, "saldo a financiar", sim.AmountFinanced, 279200)

	// Taxa mensal equivalente de 10,5% a.a.
	approx(t, "taxa mensal", sim.MonthlyRate, math.Pow(1.105, 1.0/12.0)-1)

	// Parcela Price ~R$2.456 total (spec §4.5); sua parte ~R$1.228.
	if sim.PriceInstallment < 2400 || sim.PriceInstallment > 2520 {
		t.Errorf("parcela Price fora da faixa esperada (~2456): got %.2f", sim.PriceInstallment)
	}
	approx(t, "parcela Price sua parte", sim.PriceShare, round2(sim.PriceInstallment*0.5))

	// SAC 1ª > Price (decrescente começa mais alta) ~R$3.108 total (spec §4.5).
	if sim.SacFirst <= sim.PriceInstallment {
		t.Errorf("SAC inicial (%.2f) deveria ser maior que Price (%.2f)", sim.SacFirst, sim.PriceInstallment)
	}

	// Capacidade = sobra média + entrada mensal sua parte = 1778,90 + 900 = 2678,90.
	approx(t, "capacidade", sim.Capacity, 2678.90)

	// Sua parte do Price (~1228) cabe na capacidade (~2679) → CABE.
	if !sim.FitsPrice {
		t.Errorf("parcela Price (sua parte %.2f) deveria caber na capacidade %.2f", sim.PriceShare, sim.Capacity)
	}
	if !strings.Contains(sim.Verdict, "CABE") {
		t.Errorf("veredito deveria indicar viabilidade: %q", sim.Verdict)
	}
}

func TestComputeFinancingUnaffordable(t *testing.T) {
	settings, _ := planilhaFixture()
	// Capacidade baixíssima (sobra média negativa) → não cabe.
	sim := computeFinancing(settings, -2000)
	if sim.FitsPrice {
		t.Error("com sobra média negativa a parcela não deveria caber")
	}
	if !strings.Contains(sim.Verdict, "NÃO cabe") {
		t.Errorf("veredito deveria indicar inviabilidade: %q", sim.Verdict)
	}
}

func TestComputeForecastEmptyWorkspace(t *testing.T) {
	res := computeForecast(ProjectionSettings{SharePct: 1}, nil, nil, month(2026, 7), month(2027, 6))
	if len(res.Months) != 12 {
		t.Fatalf("meses: got %d, want 12", len(res.Months))
	}
	if res.Summary.FinalCash != 0 || res.Summary.MonthsNetNegative != 0 {
		t.Errorf("workspace vazio deve projetar zeros: %+v", res.Summary)
	}
}
