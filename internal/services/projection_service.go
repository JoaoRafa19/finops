package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"finops/internal/store"
)

// Kinds de compromisso — espelham o CHECK da tabela commitments.
const (
	CommitmentInstallment  = "installment"
	CommitmentSubscription = "subscription"
	CommitmentFixed        = "fixed"
	CommitmentIncome       = "income"
	CommitmentOneOff       = "one_off"
)

var commitmentKinds = map[string]bool{
	CommitmentInstallment:  true,
	CommitmentSubscription: true,
	CommitmentFixed:        true,
	CommitmentIncome:       true,
	CommitmentOneOff:       true,
}

// Commitment é o tipo de domínio (float64/ponteiros) — a conversão de/para os
// tipos sqlc (NUMERIC string, sql.Null*) acontece só na borda do service.
type Commitment struct {
	ID           int64
	AccountID    *int64
	AccountName  string
	Name         string
	Kind         string
	MonthlyValue float64
	StartMonth   time.Time
	EndMonth     *time.Time
	Notes        string
}

// ProjectionSettings reúne as premissas globais (Premissas + parâmetros do
// Simulador de financiamento).
type ProjectionSettings struct {
	MonthlyIncome   float64
	VariableExpense float64
	OpeningBalance  float64
	HorizonStart    *time.Time
	// Simulador
	PropertyValue       float64
	DownPaymentMonthly  float64
	DownPaymentMonths   int
	FinancingAnnualRate float64
	FinancingTermYears  int
	SharePct            float64
}

// MonthProjection é uma coluna do demonstrativo mês a mês (RN-04/05/06).
type MonthProjection struct {
	Month        time.Time
	Income       float64
	FixedOut     float64
	VariableOut  float64
	TotalOut     float64
	Net          float64
	Cumulative   float64
	CommittedPct float64 // saídas / entradas (item 14)
	CardTotals   map[int64]float64
	VariableIsOverride bool // true quando o mês tem override manual (célula azul)
}

type Milestone struct {
	Month time.Time
	Label string
}

// ProjectionSummary — KPIs do dashboard Resumo.
type ProjectionSummary struct {
	AvgNet            float64
	WorstNet          float64
	WorstMonth        time.Time
	BestNet           float64
	BestMonth         time.Time
	FinalCash         float64
	LowestCash        float64   // o "vale" (menor caixa acumulado)
	LowestCashMonth   time.Time
	MonthsNetNegative int
	MonthsCashNegative int
	FirstCommittedPct  float64 // % comprometida no 1º mês do horizonte
	LastCommittedPct   float64 // % comprometida no último mês (mostra desalavancagem)
}

type ProjectionResult struct {
	Months     []MonthProjection
	CardNames  map[int64]string
	Summary    ProjectionSummary
	Milestones []Milestone
}

// FinancingSimulation — resultado do módulo Simulador (4.5 / RN-07).
type FinancingSimulation struct {
	AmountFinanced   float64
	MonthlyRate      float64
	PriceInstallment float64 // parcela fixa (Price/PMT), total do casal
	SacFirst         float64 // 1ª parcela SAC (maior), total do casal
	SacLast          float64 // última parcela SAC
	PriceShare       float64 // sua parte da parcela Price
	SacFirstShare    float64 // sua parte da 1ª SAC
	Capacity         float64 // sobra média + entrada mensal atual (sua parte)
	FitsPrice        bool
	FitsSac          bool
	Verdict          string
}

type ProjectionService interface {
	Forecast(ctx context.Context, userID int64, from, to time.Time) (ProjectionResult, error)
	Simulate(ctx context.Context, userID int64, from, to time.Time) (FinancingSimulation, error)
	ListCommitments(ctx context.Context, userID int64) ([]Commitment, error)
	GetCommitment(ctx context.Context, userID, id int64) (Commitment, error)
	CreateCommitment(ctx context.Context, userID int64, c Commitment) (Commitment, error)
	UpdateCommitment(ctx context.Context, userID int64, c Commitment) error
	DeleteCommitment(ctx context.Context, userID, id int64) error
	GetSettings(ctx context.Context, userID int64) (ProjectionSettings, error)
	SaveSettings(ctx context.Context, userID int64, s ProjectionSettings) error
	SetVariableOverride(ctx context.Context, userID int64, month time.Time, value float64) error
	ClearVariableOverride(ctx context.Context, userID int64, month time.Time) error
}

type PGProjectionService struct {
	db *store.Queries
}

func NewPGProjectionService(db *store.Queries) ProjectionService {
	return &PGProjectionService{db: db}
}

// monthOf normaliza qualquer data para o dia 1 do mês (contrato de start/end_month).
func monthOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func parseNumeric(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func formatNumeric2(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// activeIn: commitment vigente no mês m (m já normalizado para dia 1).
func (c Commitment) activeIn(m time.Time) bool {
	if c.StartMonth.After(m) {
		return false
	}
	return c.EndMonth == nil || !c.EndMonth.Before(m)
}

// computeForecast é o motor da projeção — função pura, sem DB, para ser testável
// contra os números da planilha. overrides mapeia mês (YYYY-MM) → gasto variável.
func computeForecast(s ProjectionSettings, cs []Commitment, overrides map[string]float64, from, to time.Time) ProjectionResult {
	from, to = monthOf(from), monthOf(to)

	res := ProjectionResult{CardNames: map[int64]string{}}
	cumulative := s.OpeningBalance

	for m := from; !m.After(to); m = m.AddDate(0, 1, 0) {
		mp := MonthProjection{Month: m, CardTotals: map[int64]float64{}}
		mp.Income = s.MonthlyIncome
		if v, ok := overrides[m.Format("2006-01")]; ok {
			mp.VariableOut = v
			mp.VariableIsOverride = true
		} else {
			mp.VariableOut = s.VariableExpense
		}
		for _, c := range cs {
			if !c.activeIn(m) {
				continue
			}
			if c.Kind == CommitmentIncome {
				mp.Income += c.MonthlyValue
				continue
			}
			mp.FixedOut += c.MonthlyValue
			if c.AccountID != nil {
				mp.CardTotals[*c.AccountID] += c.MonthlyValue
				res.CardNames[*c.AccountID] = c.AccountName
			}
		}
		mp.Income = round2(mp.Income)
		mp.FixedOut = round2(mp.FixedOut)
		mp.TotalOut = round2(mp.FixedOut + mp.VariableOut)
		mp.Net = round2(mp.Income - mp.TotalOut)
		cumulative = round2(cumulative + mp.Net)
		mp.Cumulative = cumulative
		if mp.Income > 0 {
			mp.CommittedPct = mp.TotalOut / mp.Income
		}
		res.Months = append(res.Months, mp)
	}

	res.Summary = summarize(res.Months)
	res.Milestones = milestones(cs, res.Months, from, to)
	return res
}

func summarize(months []MonthProjection) ProjectionSummary {
	var sum ProjectionSummary
	if len(months) == 0 {
		return sum
	}
	var total float64
	sum.WorstNet, sum.BestNet = math.Inf(1), math.Inf(-1)
	sum.LowestCash = math.Inf(1)
	for _, m := range months {
		total += m.Net
		if m.Net < sum.WorstNet {
			sum.WorstNet, sum.WorstMonth = m.Net, m.Month
		}
		if m.Net > sum.BestNet {
			sum.BestNet, sum.BestMonth = m.Net, m.Month
		}
		if m.Cumulative < sum.LowestCash {
			sum.LowestCash, sum.LowestCashMonth = m.Cumulative, m.Month
		}
		if m.Net < 0 {
			sum.MonthsNetNegative++
		}
		if m.Cumulative < 0 {
			sum.MonthsCashNegative++
		}
	}
	sum.AvgNet = round2(total / float64(len(months)))
	sum.FinalCash = months[len(months)-1].Cumulative
	sum.FirstCommittedPct = months[0].CommittedPct
	sum.LastCommittedPct = months[len(months)-1].CommittedPct
	return sum
}

// milestones deriva marcos determinísticos: fim de cada compromisso dentro do
// horizonte, entradas pontuais e meses no vermelho.
func milestones(cs []Commitment, months []MonthProjection, from, to time.Time) []Milestone {
	var ms []Milestone
	for _, c := range cs {
		if c.Kind == CommitmentOneOff {
			if !c.StartMonth.Before(from) && !c.StartMonth.After(to) {
				ms = append(ms, Milestone{c.StartMonth, fmt.Sprintf("Evento único: %s", c.Name)})
			}
			continue
		}
		if c.EndMonth == nil || c.EndMonth.Before(from) || c.EndMonth.After(to) {
			continue
		}
		switch c.Kind {
		case CommitmentIncome:
			if c.StartMonth.Equal(*c.EndMonth) {
				ms = append(ms, Milestone{*c.EndMonth, fmt.Sprintf("Entrada pontual: %s", c.Name)})
			} else {
				ms = append(ms, Milestone{*c.EndMonth, fmt.Sprintf("Último recebimento: %s", c.Name)})
			}
		case CommitmentInstallment:
			ms = append(ms, Milestone{*c.EndMonth, fmt.Sprintf("Última parcela: %s", c.Name)})
		default:
			ms = append(ms, Milestone{*c.EndMonth, fmt.Sprintf("Fim de %s", c.Name)})
		}
	}
	for _, m := range months {
		if m.Net < 0 || m.Cumulative < 0 {
			ms = append(ms, Milestone{m.Month, "Mês no vermelho"})
		}
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].Month.Before(ms[j].Month) })
	return ms
}

// computeFinancing implementa o Simulador (RN-07). Puro para ser testável.
// avgNet e currentDownPaymentShare vêm da projeção (sobra média e entrada atual).
func computeFinancing(s ProjectionSettings, avgNet float64) FinancingSimulation {
	var sim FinancingSimulation
	share := s.SharePct
	if share <= 0 {
		share = 1
	}

	// Saldo a financiar = valor do imóvel − entrada já paga durante a obra.
	sim.AmountFinanced = round2(s.PropertyValue - s.DownPaymentMonthly*float64(s.DownPaymentMonths))
	if sim.AmountFinanced < 0 {
		sim.AmountFinanced = 0
	}

	n := s.FinancingTermYears * 12
	// Taxa mensal equivalente = (1 + anual)^(1/12) − 1.
	if s.FinancingAnnualRate > 0 {
		sim.MonthlyRate = math.Pow(1+s.FinancingAnnualRate, 1.0/12.0) - 1
	}

	if n > 0 && sim.AmountFinanced > 0 {
		i := sim.MonthlyRate
		if i > 0 {
			// Parcela Price (fixa): PMT = saldo·i / (1 − (1+i)^-n).
			sim.PriceInstallment = round2(sim.AmountFinanced * i / (1 - math.Pow(1+i, float64(-n))))
		} else {
			sim.PriceInstallment = round2(sim.AmountFinanced / float64(n))
		}
		// SAC: amortização constante + juros sobre o saldo.
		amort := sim.AmountFinanced / float64(n)
		sim.SacFirst = round2(amort + sim.AmountFinanced*i)
		sim.SacLast = round2(amort + amort*i)
	}

	sim.PriceShare = round2(sim.PriceInstallment * share)
	sim.SacFirstShare = round2(sim.SacFirst * share)

	// Capacidade = sobra média + entrada mensal atual (sua parte), porque em 2029
	// a entrada deixa de existir e é substituída pela parcela.
	sim.Capacity = round2(avgNet + s.DownPaymentMonthly*share)
	sim.FitsPrice = sim.PriceShare <= sim.Capacity
	sim.FitsSac = sim.SacFirstShare <= sim.Capacity

	switch {
	case sim.PriceInstallment == 0:
		sim.Verdict = "Configure os parâmetros do imóvel e do financiamento nas Premissas para simular."
	case sim.FitsSac:
		sim.Verdict = "CABE — a parcela (mesmo no SAC inicial, mais alta) cabe na sua capacidade projetada."
	case sim.FitsPrice:
		sim.Verdict = "CABE no Price — a parcela fixa cabe, mas a 1ª parcela do SAC ultrapassa a capacidade. Prefira Price ou aumente a folga."
	default:
		sim.Verdict = "NÃO cabe — a parcela supera a capacidade projetada; precisa de aumento de renda, prazo maior ou mais entrada."
	}
	return sim
}

func (p *PGProjectionService) loadForecastInputs(ctx context.Context, userID int64) (ProjectionSettings, []Commitment, map[string]float64, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return ProjectionSettings{}, nil, nil, fmt.Errorf("get workspace: %w", err)
	}
	settings, err := p.getSettingsByWorkspace(ctx, ws.ID)
	if err != nil {
		return ProjectionSettings{}, nil, nil, err
	}
	rows, err := p.db.ListCommitments(ctx, ws.ID)
	if err != nil {
		return ProjectionSettings{}, nil, nil, fmt.Errorf("list commitments: %w", err)
	}
	cs := make([]Commitment, 0, len(rows))
	for _, r := range rows {
		cs = append(cs, commitmentFromRow(r))
	}
	ovRows, err := p.db.ListVariableExpenseOverrides(ctx, ws.ID)
	if err != nil {
		return ProjectionSettings{}, nil, nil, fmt.Errorf("list overrides: %w", err)
	}
	overrides := make(map[string]float64, len(ovRows))
	for _, o := range ovRows {
		overrides[monthOf(o.Month).Format("2006-01")] = parseNumeric(o.Value)
	}
	return settings, cs, overrides, nil
}

func (p *PGProjectionService) Forecast(ctx context.Context, userID int64, from, to time.Time) (ProjectionResult, error) {
	settings, cs, overrides, err := p.loadForecastInputs(ctx, userID)
	if err != nil {
		return ProjectionResult{}, err
	}
	return computeForecast(settings, cs, overrides, from, to), nil
}

func (p *PGProjectionService) Simulate(ctx context.Context, userID int64, from, to time.Time) (FinancingSimulation, error) {
	settings, cs, overrides, err := p.loadForecastInputs(ctx, userID)
	if err != nil {
		return FinancingSimulation{}, err
	}
	res := computeForecast(settings, cs, overrides, from, to)
	return computeFinancing(settings, res.Summary.AvgNet), nil
}

func (p *PGProjectionService) ListCommitments(ctx context.Context, userID int64) ([]Commitment, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	rows, err := p.db.ListCommitments(ctx, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list commitments: %w", err)
	}
	cs := make([]Commitment, 0, len(rows))
	for _, r := range rows {
		cs = append(cs, commitmentFromRow(r))
	}
	return cs, nil
}

func (p *PGProjectionService) GetCommitment(ctx context.Context, userID, id int64) (Commitment, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return Commitment{}, fmt.Errorf("get workspace: %w", err)
	}
	row, err := p.db.GetCommitmentByWorkspaceAndID(ctx, store.GetCommitmentByWorkspaceAndIDParams{
		WorkspaceID: ws.ID,
		ID:          id,
	})
	if err != nil {
		return Commitment{}, fmt.Errorf("get commitment: %w", err)
	}
	c := Commitment{
		ID:           row.ID,
		Name:         row.Name,
		Kind:         row.Kind,
		MonthlyValue: parseNumeric(row.MonthlyValue),
		StartMonth:   monthOf(row.StartMonth),
		Notes:        row.Notes.String,
	}
	if row.AccountID.Valid {
		c.AccountID = &row.AccountID.Int64
	}
	if row.EndMonth.Valid {
		end := monthOf(row.EndMonth.Time)
		c.EndMonth = &end
	}
	return c, nil
}

func validateCommitment(c Commitment) error {
	if c.Name == "" {
		return errors.New("nome é obrigatório")
	}
	if !commitmentKinds[c.Kind] {
		return fmt.Errorf("tipo inválido: %s", c.Kind)
	}
	if c.MonthlyValue <= 0 {
		return errors.New("valor mensal deve ser maior que zero")
	}
	if c.EndMonth != nil && c.EndMonth.Before(c.StartMonth) {
		return errors.New("mês final não pode ser anterior ao inicial")
	}
	return nil
}

func (p *PGProjectionService) CreateCommitment(ctx context.Context, userID int64, c Commitment) (Commitment, error) {
	if err := validateCommitment(c); err != nil {
		return Commitment{}, err
	}
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return Commitment{}, fmt.Errorf("get workspace: %w", err)
	}
	row, err := p.db.CreateCommitment(ctx, store.CreateCommitmentParams{
		WorkspaceID:  ws.ID,
		AccountID:    nullInt64(c.AccountID),
		Name:         c.Name,
		Kind:         c.Kind,
		MonthlyValue: formatNumeric2(c.MonthlyValue),
		StartMonth:   monthOf(c.StartMonth),
		EndMonth:     nullMonth(c.EndMonth),
		Notes:        nullString(c.Notes),
	})
	if err != nil {
		return Commitment{}, fmt.Errorf("create commitment: %w", err)
	}
	c.ID = row.ID
	return c, nil
}

func (p *PGProjectionService) UpdateCommitment(ctx context.Context, userID int64, c Commitment) error {
	if err := validateCommitment(c); err != nil {
		return err
	}
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}
	if err := p.db.UpdateCommitment(ctx, store.UpdateCommitmentParams{
		WorkspaceID:  ws.ID,
		ID:           c.ID,
		AccountID:    nullInt64(c.AccountID),
		Name:         c.Name,
		Kind:         c.Kind,
		MonthlyValue: formatNumeric2(c.MonthlyValue),
		StartMonth:   monthOf(c.StartMonth),
		EndMonth:     nullMonth(c.EndMonth),
		Notes:        nullString(c.Notes),
	}); err != nil {
		return fmt.Errorf("update commitment: %w", err)
	}
	return nil
}

func (p *PGProjectionService) DeleteCommitment(ctx context.Context, userID, id int64) error {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}
	if err := p.db.DeleteCommitment(ctx, store.DeleteCommitmentParams{WorkspaceID: ws.ID, ID: id}); err != nil {
		return fmt.Errorf("delete commitment: %w", err)
	}
	return nil
}

func (p *PGProjectionService) GetSettings(ctx context.Context, userID int64) (ProjectionSettings, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return ProjectionSettings{}, fmt.Errorf("get workspace: %w", err)
	}
	return p.getSettingsByWorkspace(ctx, ws.ID)
}

// getSettingsByWorkspace devolve zeros para workspace sem settings — a tela
// funciona sem setup prévio (share_pct default 1).
func (p *PGProjectionService) getSettingsByWorkspace(ctx context.Context, workspaceID int64) (ProjectionSettings, error) {
	row, err := p.db.GetProjectionSettings(ctx, workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectionSettings{SharePct: 1}, nil
	}
	if err != nil {
		return ProjectionSettings{}, fmt.Errorf("get projection settings: %w", err)
	}
	s := ProjectionSettings{
		MonthlyIncome:       parseNumeric(row.MonthlyIncome),
		VariableExpense:     parseNumeric(row.VariableExpense),
		OpeningBalance:      parseNumeric(row.OpeningBalance),
		PropertyValue:       parseNumeric(row.PropertyValue),
		DownPaymentMonthly:  parseNumeric(row.DownPaymentMonthly),
		DownPaymentMonths:   int(row.DownPaymentMonths),
		FinancingAnnualRate: parseNumeric(row.FinancingAnnualRate),
		FinancingTermYears:  int(row.FinancingTermYears),
		SharePct:            parseNumeric(row.SharePct),
	}
	if row.HorizonStart.Valid {
		h := monthOf(row.HorizonStart.Time)
		s.HorizonStart = &h
	}
	if s.SharePct <= 0 {
		s.SharePct = 1
	}
	return s, nil
}

func (p *PGProjectionService) SaveSettings(ctx context.Context, userID int64, s ProjectionSettings) error {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}
	if _, err := p.db.UpsertProjectionSettings(ctx, store.UpsertProjectionSettingsParams{
		WorkspaceID:         ws.ID,
		MonthlyIncome:       formatNumeric2(s.MonthlyIncome),
		VariableExpense:     formatNumeric2(s.VariableExpense),
		OpeningBalance:      formatNumeric2(s.OpeningBalance),
		HorizonStart:        nullMonth(s.HorizonStart),
		PropertyValue:       formatNumeric2(s.PropertyValue),
		DownPaymentMonthly:  formatNumeric2(s.DownPaymentMonthly),
		DownPaymentMonths:   int32(s.DownPaymentMonths),
		FinancingAnnualRate: strconv.FormatFloat(s.FinancingAnnualRate, 'f', 4, 64),
		FinancingTermYears:  int32(s.FinancingTermYears),
		SharePct:            strconv.FormatFloat(s.SharePct, 'f', 4, 64),
	}); err != nil {
		return fmt.Errorf("save projection settings: %w", err)
	}
	return nil
}

func (p *PGProjectionService) SetVariableOverride(ctx context.Context, userID int64, month time.Time, value float64) error {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}
	if err := p.db.UpsertVariableExpenseOverride(ctx, store.UpsertVariableExpenseOverrideParams{
		WorkspaceID: ws.ID,
		Month:       monthOf(month),
		Value:       formatNumeric2(value),
	}); err != nil {
		return fmt.Errorf("set variable override: %w", err)
	}
	return nil
}

func (p *PGProjectionService) ClearVariableOverride(ctx context.Context, userID int64, month time.Time) error {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}
	if err := p.db.DeleteVariableExpenseOverride(ctx, store.DeleteVariableExpenseOverrideParams{
		WorkspaceID: ws.ID,
		Month:       monthOf(month),
	}); err != nil {
		return fmt.Errorf("clear variable override: %w", err)
	}
	return nil
}

func commitmentFromRow(r store.ListCommitmentsRow) Commitment {
	c := Commitment{
		ID:           r.ID,
		Name:         r.Name,
		Kind:         r.Kind,
		MonthlyValue: parseNumeric(r.MonthlyValue),
		StartMonth:   monthOf(r.StartMonth),
		AccountName:  r.AccountName.String,
		Notes:        r.Notes.String,
	}
	if r.AccountID.Valid {
		c.AccountID = &r.AccountID.Int64
	}
	if r.EndMonth.Valid {
		end := monthOf(r.EndMonth.Time)
		c.EndMonth = &end
	}
	return c
}

func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullMonth(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: monthOf(*t), Valid: true}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
