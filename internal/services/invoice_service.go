package service

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"finops/internal/store"
	"finops/internal/utils"

	"github.com/ledongthuc/pdf"
)

// CommitmentProposal é um compromisso proposto pela IA a partir da fatura —
// vira Commitment de verdade só depois da confirmação do usuário.
type CommitmentProposal struct {
	Name         string
	Kind         string
	MonthlyValue float64
	StartMonth   time.Time
	EndMonth     *time.Time
	Notes        string
}

type InvoiceProposal struct {
	Transactions []ImportRow
	Commitments  []CommitmentProposal
}

type InvoiceImportResult struct {
	TransactionsInserted   int
	TransactionsDuplicated int
	CommitmentsCreated     int
	CommitmentsSkipped     int
}

type InvoiceService interface {
	// ExtractInvoice roda a IA sobre a fatura e devolve a proposta em staging
	// (uuid TTL 10min) — nada é gravado no banco até ConfirmInvoice.
	ExtractInvoice(ctx context.Context, userID, accountID int64, filename string, content []byte) (string, InvoiceProposal, error)
	ConfirmInvoice(ctx context.Context, userID, accountID int64, uuid string) (InvoiceImportResult, error)
}

// --- staging temp store (espelho do importTempStore, guarda a proposta) ---

type invoiceTempEntry struct {
	prop    InvoiceProposal
	expires time.Time
}

type invoiceTempStore struct {
	mu      sync.RWMutex
	entries map[string]invoiceTempEntry
	once    sync.Once
}

func (s *invoiceTempStore) start() {
	s.once.Do(func() {
		go func() {
			for range time.Tick(5 * time.Minute) {
				now := time.Now()
				s.mu.Lock()
				for k, e := range s.entries {
					if now.After(e.expires) {
						delete(s.entries, k)
					}
				}
				s.mu.Unlock()
			}
		}()
	})
}

func (s *invoiceTempStore) put(prop InvoiceProposal) string {
	id := newUUID()
	s.mu.Lock()
	s.entries[id] = invoiceTempEntry{prop: prop, expires: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	return id
}

func (s *invoiceTempStore) get(id string) (InvoiceProposal, bool) {
	s.mu.RLock()
	e, ok := s.entries[id]
	s.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return InvoiceProposal{}, false
	}
	return e.prop, true
}

// --- service ---

type AIInvoiceService struct {
	db        *store.Queries
	aiSvc     AIService
	importSvc ImportService
	projSvc   ProjectionService
	tmpStore  *invoiceTempStore
}

func NewAIInvoiceService(db *store.Queries, aiSvc AIService, importSvc ImportService, projSvc ProjectionService) InvoiceService {
	ts := &invoiceTempStore{entries: make(map[string]invoiceTempEntry)}
	ts.start()
	return &AIInvoiceService{db: db, aiSvc: aiSvc, importSvc: importSvc, projSvc: projSvc, tmpStore: ts}
}

const invoiceSystemPrompt = `Você é um extrator de faturas de cartão de crédito do app Finops.
Receberá o conteúdo de uma fatura. Sua tarefa:

1. Use list_commitments e find_similar_transactions para ver o que JÁ está cadastrado.
2. Para cada transação da fatura, chame stage_transactions com os lançamentos do período (não inclua os que find_similar_transactions mostrar como já existentes).
3. Para cada PARCELAMENTO (ex.: "LOJA 3/10" = parcela 3 de 10) que ainda não exista em list_commitments, chame stage_commitment com kind=installment, o valor da parcela e end_month calculado a partir da parcela atual (ex.: parcela 3/10 na fatura de 2026-07 termina em 2027-02).
4. Para cada ASSINATURA recorrente (streaming, telefonia, software) que ainda não exista, chame stage_commitment com kind=subscription e end_month vazio.
5. Ao terminar, responda apenas "OK".

Datas sempre no formato YYYY-MM-DD (transações) ou YYYY-MM (meses). Valores em reais com ponto decimal. Direção: debit para gastos, credit para estornos/pagamentos.`

func (s *AIInvoiceService) ExtractInvoice(ctx context.Context, userID, accountID int64, filename string, content []byte) (string, InvoiceProposal, error) {
	text, err := invoiceText(filename, content)
	if err != nil {
		return "", InvoiceProposal{}, err
	}

	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return "", InvoiceProposal{}, fmt.Errorf("get workspace: %w", err)
	}

	staging := &InvoiceProposal{}
	tools := s.invoiceTools(ws.ID, accountID, userID, staging)

	if _, err := s.aiSvc.RunAgentLoop(ctx, invoiceSystemPrompt, text, tools, 12); err != nil {
		return "", InvoiceProposal{}, fmt.Errorf("extração da fatura: %w", err)
	}
	if len(staging.Transactions) == 0 && len(staging.Commitments) == 0 {
		return "", InvoiceProposal{}, fmt.Errorf("a IA não encontrou lançamentos na fatura")
	}

	uuid := s.tmpStore.put(*staging)
	return uuid, *staging, nil
}

func (s *AIInvoiceService) ConfirmInvoice(ctx context.Context, userID, accountID int64, uuid string) (InvoiceImportResult, error) {
	prop, ok := s.tmpStore.get(uuid)
	if !ok {
		return InvoiceImportResult{}, fmt.Errorf("sessão de importação expirada")
	}
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return InvoiceImportResult{}, fmt.Errorf("get workspace: %w", err)
	}

	// Backstop determinístico: o dedup do preview depende da IA; o Confirm não.
	existingRows, err := s.db.ListCommitments(ctx, ws.ID)
	if err != nil {
		return InvoiceImportResult{}, fmt.Errorf("list commitments: %w", err)
	}
	existing := make([]Commitment, 0, len(existingRows))
	for _, r := range existingRows {
		existing = append(existing, commitmentFromRow(r))
	}
	deduped := dedupProposal(prop, existing)

	var res InvoiceImportResult
	res.CommitmentsSkipped = len(prop.Commitments) - len(deduped.Commitments)

	if len(deduped.Transactions) > 0 {
		// FITID sintético (hash data+valor+descrição) reusa o índice único
		// uq_tx_fitid — importar a mesma fatura duas vezes não duplica nada.
		imported, err := s.importSvc.ImportRows(ctx, userID, accountID, deduped.Transactions)
		if err != nil {
			return res, fmt.Errorf("import transactions: %w", err)
		}
		res.TransactionsInserted = imported.Inserted
		res.TransactionsDuplicated = imported.Duplicates
	}

	for _, c := range deduped.Commitments {
		commitment := Commitment{
			AccountID:    &accountID,
			Name:         c.Name,
			Kind:         c.Kind,
			MonthlyValue: c.MonthlyValue,
			StartMonth:   c.StartMonth,
			EndMonth:     c.EndMonth,
			Notes:        c.Notes,
		}
		if _, err := s.projSvc.CreateCommitment(ctx, userID, commitment); err != nil {
			return res, fmt.Errorf("create commitment %q: %w", c.Name, err)
		}
		res.CommitmentsCreated++
	}
	return res, nil
}

// dedupProposal remove da proposta o que já existe: commitments por
// name+kind+valor+start (normalizados) e transações duplicadas dentro da
// própria proposta (o banco barra repetição entre imports via FITID sintético).
func dedupProposal(prop InvoiceProposal, existing []Commitment) InvoiceProposal {
	existingKeys := make(map[string]bool, len(existing))
	for _, c := range existing {
		existingKeys[commitmentKey(c.Name, c.Kind, c.MonthlyValue, c.StartMonth)] = true
	}

	var out InvoiceProposal
	for _, c := range prop.Commitments {
		if existingKeys[commitmentKey(c.Name, c.Kind, c.MonthlyValue, c.StartMonth)] {
			continue
		}
		out.Commitments = append(out.Commitments, c)
	}

	seen := make(map[string]bool, len(prop.Transactions))
	for _, t := range prop.Transactions {
		if t.ExternalFitid == "" {
			t.ExternalFitid = syntheticFitid(t)
		}
		if seen[t.ExternalFitid] {
			continue
		}
		seen[t.ExternalFitid] = true
		out.Transactions = append(out.Transactions, t)
	}
	return out
}

func commitmentKey(name, kind string, value float64, start time.Time) string {
	return fmt.Sprintf("%s|%s|%.2f|%s", strings.ToLower(strings.TrimSpace(name)), kind, value, monthOf(start).Format("2006-01"))
}

func syntheticFitid(t ImportRow) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%.2f|%s|%s", t.PostedOn.Format("2006-01-02"), t.Amount, t.Direction, strings.ToLower(strings.TrimSpace(t.Description)))))
	return fmt.Sprintf("inv-%x", h[:8])
}

// invoiceText extrai o texto do arquivo: PDF via ledongthuc/pdf, resto é
// tratado como texto (CSV/TXT). Imagens ficam para quando o LLM tiver vision.
func invoiceText(filename string, content []byte) (string, error) {
	name := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(name, ".pdf"):
		reader, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			return "", fmt.Errorf("ler PDF: %w", err)
		}
		var sb strings.Builder
		for i := 1; i <= reader.NumPage(); i++ {
			page := reader.Page(i)
			if page.V.IsNull() {
				continue
			}
			text, err := page.GetPlainText(nil)
			if err != nil {
				continue
			}
			sb.WriteString(text)
			sb.WriteString("\n")
		}
		if strings.TrimSpace(sb.String()) == "" {
			return "", fmt.Errorf("PDF sem texto extraível (fatura escaneada?)")
		}
		return sb.String(), nil
	case strings.HasSuffix(name, ".png"), strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		return "", fmt.Errorf("imagens ainda não são suportadas — envie o PDF ou CSV da fatura")
	default:
		return string(content), nil
	}
}

// invoiceTools: leitura consulta o banco; escrita vai para o staging, nunca
// direto para o DB — o usuário confirma o preview antes de gravar.
func (s *AIInvoiceService) invoiceTools(workspaceID, accountID, userID int64, staging *InvoiceProposal) []financialTool {
	return []financialTool{
		{
			schema: tool("list_commitments", "Lista os compromissos (parcelamentos, assinaturas) já cadastrados — não proponha de novo o que já existe.", params(nil)),
			handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
				cs, err := s.projSvc.ListCommitments(ctx, userID)
				if err != nil {
					return "", err
				}
				if len(cs) == 0 {
					return "Nenhum compromisso cadastrado.", nil
				}
				var sb strings.Builder
				for _, c := range cs {
					end := "sem fim"
					if c.EndMonth != nil {
						end = c.EndMonth.Format("2006-01")
					}
					fmt.Fprintf(&sb, "- %s (%s) %s/mês, %s até %s\n", c.Name, c.Kind, utils.FormatMoney(c.MonthlyValue), c.StartMonth.Format("2006-01"), end)
				}
				return sb.String(), nil
			},
		},
		{
			schema: tool("find_similar_transactions",
				"Busca transações já registradas com o mesmo valor em uma janela de ±3 dias — use para não duplicar lançamentos.",
				params(map[string]any{
					"date":   prop("string", "Data do lançamento (YYYY-MM-DD)"),
					"amount": prop("number", "Valor absoluto do lançamento"),
				}, "date", "amount"),
			),
			handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				var p struct {
					Date   string  `json:"date"`
					Amount float64 `json:"amount"`
				}
				if err := json.Unmarshal(args, &p); err != nil {
					return "", err
				}
				day, err := time.Parse("2006-01-02", p.Date)
				if err != nil {
					return "", fmt.Errorf("data inválida: %s", p.Date)
				}
				rows, err := s.db.FindSimilarTransactions(ctx, store.FindSimilarTransactionsParams{
					WorkspaceID: workspaceID,
					AccountID:   accountID,
					Amount:      formatNumeric2(p.Amount),
					PostedOn:    day.AddDate(0, 0, -3),
					PostedOn_2:  day.AddDate(0, 0, 3),
				})
				if err != nil {
					return "", err
				}
				if len(rows) == 0 {
					return "Nenhuma transação parecida encontrada.", nil
				}
				var sb strings.Builder
				for _, r := range rows {
					fmt.Fprintf(&sb, "- %s %s %s (%s)\n", r.PostedOn.Format("2006-01-02"), r.Amount, r.Description, r.Direction)
				}
				return sb.String(), nil
			},
		},
		{
			schema: tool("stage_transactions",
				"Adiciona lançamentos da fatura à proposta de importação (itens malformados são ignorados).",
				params(map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": "Lançamentos da fatura",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"date":        prop("string", "Data (YYYY-MM-DD)"),
								"description": prop("string", "Descrição do lançamento"),
								"amount":      prop("number", "Valor absoluto"),
								"direction":   map[string]any{"type": "string", "enum": []string{"debit", "credit"}, "description": "debit=gasto, credit=estorno/pagamento"},
							},
						},
					},
				}, "items"),
			),
			handler: func(_ context.Context, args json.RawMessage) (string, error) {
				var p struct {
					Items []struct {
						Date        string  `json:"date"`
						Description string  `json:"description"`
						Amount      float64 `json:"amount"`
						Direction   string  `json:"direction"`
					} `json:"items"`
				}
				if err := json.Unmarshal(args, &p); err != nil {
					return "", err
				}
				added, skipped := 0, 0
				for _, it := range p.Items {
					day, err := time.Parse("2006-01-02", it.Date)
					if err != nil || it.Amount <= 0 || strings.TrimSpace(it.Description) == "" || (it.Direction != "debit" && it.Direction != "credit") {
						skipped++
						continue
					}
					row := ImportRow{
						PostedOn:    day,
						Description: strings.TrimSpace(it.Description),
						Amount:      it.Amount,
						Direction:   it.Direction,
					}
					row.ExternalFitid = syntheticFitid(row)
					staging.Transactions = append(staging.Transactions, row)
					added++
				}
				return fmt.Sprintf("%d lançamento(s) adicionados à proposta, %d ignorados.", added, skipped), nil
			},
		},
		{
			schema: tool("stage_commitment",
				"Adiciona um parcelamento ou assinatura à proposta (vira compromisso após confirmação do usuário).",
				params(map[string]any{
					"name":          prop("string", "Nome do compromisso, ex: 'KaBuM (10x)'"),
					"kind":          map[string]any{"type": "string", "enum": []string{"installment", "subscription"}, "description": "installment=parcelamento, subscription=assinatura"},
					"monthly_value": prop("number", "Valor mensal"),
					"start_month":   prop("string", "Primeiro mês (YYYY-MM)"),
					"end_month":     prop("string", "Último mês (YYYY-MM); vazio para assinatura sem fim"),
					"notes":         prop("string", "Observações, ex: 'parcela 3/10 na fatura de julho'"),
				}, "name", "kind", "monthly_value", "start_month"),
			),
			handler: func(_ context.Context, args json.RawMessage) (string, error) {
				var p struct {
					Name         string  `json:"name"`
					Kind         string  `json:"kind"`
					MonthlyValue float64 `json:"monthly_value"`
					StartMonth   string  `json:"start_month"`
					EndMonth     string  `json:"end_month"`
					Notes        string  `json:"notes"`
				}
				if err := json.Unmarshal(args, &p); err != nil {
					return "", err
				}
				start, err := time.Parse("2006-01", p.StartMonth)
				if err != nil || strings.TrimSpace(p.Name) == "" || p.MonthlyValue <= 0 || (p.Kind != CommitmentInstallment && p.Kind != CommitmentSubscription) {
					return "Compromisso ignorado: dados inválidos.", nil
				}
				cp := CommitmentProposal{
					Name:         strings.TrimSpace(p.Name),
					Kind:         p.Kind,
					MonthlyValue: p.MonthlyValue,
					StartMonth:   start,
					Notes:        strings.TrimSpace(p.Notes),
				}
				if p.EndMonth != "" {
					if end, err := time.Parse("2006-01", p.EndMonth); err == nil && !end.Before(start) {
						cp.EndMonth = &end
					}
				}
				staging.Commitments = append(staging.Commitments, cp)
				return fmt.Sprintf("Compromisso %q adicionado à proposta.", cp.Name), nil
			},
		},
	}
}
