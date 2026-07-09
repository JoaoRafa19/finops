package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"finops/internal/models"
	"finops/internal/store"
)

// resolveAccount casa o nome informado com uma conta (ou usa a única existente).
func resolveAccount(accounts []store.Account, name string) (store.Account, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		if len(accounts) == 1 {
			return accounts[0], true
		}
		return store.Account{}, false
	}
	n := normalizeDesc(name)
	// match exato normalizado primeiro, depois contains.
	for _, a := range accounts {
		if normalizeDesc(a.Name) == n {
			return a, true
		}
	}
	for _, a := range accounts {
		an := normalizeDesc(a.Name)
		if strings.Contains(an, n) || strings.Contains(n, an) {
			return a, true
		}
	}
	return store.Account{}, false
}

func resolveCategory(cats []store.Category, name string) (int64, string, bool) {
	n := normalizeDesc(name)
	for _, c := range cats {
		if normalizeDesc(c.Name) == n {
			return c.ID, c.Name, true
		}
	}
	for _, c := range cats {
		cn := normalizeDesc(c.Name)
		if strings.Contains(cn, n) || strings.Contains(n, cn) {
			return c.ID, c.Name, true
		}
	}
	return 0, "", false
}

func asDuplicate(err error, target **DuplicateError) bool {
	return errors.As(err, target)
}

const chatPendingTxKeyFmt = "finops:chat:pendingtx:%d"
const chatPendingTxTTL = 15 * time.Minute

// stagedTransaction é a transação montada pelo chat, guardada até a confirmação.
type stagedTransaction struct {
	AccountID    int64   `json:"account_id"`
	AccountName  string  `json:"account_name"`
	CategoryID   *int64  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	PostedOn     string  `json:"posted_on"` // YYYY-MM-DD
	Description  string  `json:"description"`
	Amount       float64 `json:"amount"`
	Direction    string  `json:"direction"`
}

// transactionWriteTools são as ferramentas de escrita do chat: stage (monta e
// valida, sem gravar) e commit (grava a pendente após o usuário confirmar).
func (s *OllamaAgentChatService) transactionWriteTools(userID int64) []financialTool {
	return []financialTool{
		s.stageTransactionTool(userID),
		s.commitTransactionTool(userID),
	}
}

func (s *OllamaAgentChatService) pendingTxKey(userID int64) string {
	return fmt.Sprintf(chatPendingTxKeyFmt, userID)
}

func (s *OllamaAgentChatService) stageTransactionTool(userID int64) financialTool {
	return financialTool{
		schema: tool("stage_transaction",
			"Prepara (NÃO grava) uma transação a partir do que o usuário descreveu. Resolve a conta e a categoria pelo nome, valida e checa duplicatas. Depois de chamar, MOSTRE os detalhes ao usuário e peça confirmação explícita antes de chamar commit_transaction.",
			params(map[string]any{
				"description": prop("string", "Descrição da transação, ex: 'Mercado Extra'"),
				"amount":      prop("number", "Valor absoluto (sempre positivo)"),
				"direction":   map[string]any{"type": "string", "enum": []string{"debit", "credit"}, "description": "debit=gasto/saída, credit=receita/entrada"},
				"date":        prop("string", "Data no formato YYYY-MM-DD (opcional; default hoje)"),
				"account":     prop("string", "Nome da conta (opcional se o usuário tiver só uma)"),
				"category":    prop("string", "Nome da categoria (opcional)"),
			}, "description", "amount", "direction"),
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Description string  `json:"description"`
				Amount      float64 `json:"amount"`
				Direction   string  `json:"direction"`
				Date        string  `json:"date"`
				Account     string  `json:"account"`
				Category    string  `json:"category"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", err
			}
			p.Description = strings.TrimSpace(p.Description)
			if p.Description == "" {
				return "Descrição é obrigatória.", nil
			}
			if p.Amount <= 0 {
				return "O valor precisa ser maior que zero.", nil
			}
			if p.Direction != "debit" && p.Direction != "credit" {
				return "Direção inválida — use 'debit' (gasto) ou 'credit' (receita).", nil
			}

			postedOn := time.Now()
			if p.Date != "" {
				// llama às vezes manda "hoje"/"ontem" em vez de YYYY-MM-DD; se não
				// parsear, assume hoje (o usuário confere a data no preview).
				if t, err := time.Parse("2006-01-02", p.Date); err == nil {
					postedOn = t
				}
			}

			// Resolve conta por nome (ou a única existente).
			accounts, err := s.accountSvc.ListByUser(ctx, userID)
			if err != nil {
				return "", err
			}
			if len(accounts) == 0 {
				return "Você ainda não tem contas cadastradas. Crie uma conta antes de registrar transações.", nil
			}
			acc, ok := resolveAccount(accounts, p.Account)
			if !ok {
				var names []string
				for _, a := range accounts {
					names = append(names, a.Name)
				}
				return fmt.Sprintf("Não identifiquei a conta %q. Contas disponíveis: %s. Pergunte ao usuário qual usar.", p.Account, strings.Join(names, ", ")), nil
			}

			// Categoria é opcional.
			var categoryID *int64
			var categoryName string
			if p.Category != "" {
				cats, err := s.categorySvc.GetCategories(ctx, userID)
				if err == nil {
					if id, name, ok := resolveCategory(cats, p.Category); ok {
						categoryID = &id
						categoryName = name
					}
				}
			}

			// Dedup: bloqueia antes mesmo de encenar.
			if match, dup, err := s.transactionSvc.FindDuplicate(ctx, userID, acc.ID, postedOn, p.Amount, p.Direction, p.Description); err == nil && dup {
				return fmt.Sprintf("⚠ Já existe uma transação parecida: %q de %s em %s. Não vou duplicar. Se for mesmo uma nova, peça ao usuário para ajustar data/descrição.",
					match.Description, formatNumeric2(match.Amount), match.PostedOn.Format("02/01/2006")), nil
			}

			staged := stagedTransaction{
				AccountID:    acc.ID,
				AccountName:  acc.Name,
				CategoryID:   categoryID,
				CategoryName: categoryName,
				PostedOn:     postedOn.Format("2006-01-02"),
				Description:  p.Description,
				Amount:       p.Amount,
				Direction:    p.Direction,
			}
			raw, _ := json.Marshal(staged)
			if err := s.rdb.Set(ctx, s.pendingTxKey(userID), raw, chatPendingTxTTL).Err(); err != nil {
				return "", fmt.Errorf("guardar transação pendente: %w", err)
			}

			kind := "Gasto"
			if p.Direction == "credit" {
				kind = "Receita"
			}
			cat := categoryName
			if cat == "" {
				cat = "sem categoria"
			}
			// Resumo pronto para o assistente copiar verbatim. O modelo (llama) tende
			// a resumir demais; entregamos o texto final exato que ele deve enviar.
			return fmt.Sprintf("Transação PREPARADA (ainda NÃO gravada). RESPONDA AO USUÁRIO copiando EXATAMENTE o bloco abaixo, sem omitir nada:\n\n"+
				"**Confira antes de gravar:**\n- %s: R$ %s\n- Conta: %s\n- Categoria: %s\n- Data: %s\n\nConfirma? (responda \"sim\" para gravar)",
				kind, formatNumeric2(p.Amount), acc.Name, cat, postedOn.Format("02/01/2006")), nil
		},
	}
}

func (s *OllamaAgentChatService) commitTransactionTool(userID int64) financialTool {
	return financialTool{
		schema: tool("commit_transaction",
			"Grava de fato a transação preparada por stage_transaction. Chame APENAS depois que o usuário confirmar explicitamente.",
			params(nil),
		),
		handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			raw, err := s.rdb.Get(ctx, s.pendingTxKey(userID)).Bytes()
			if err != nil {
				return "Não há nenhuma transação preparada para gravar. Use stage_transaction primeiro.", nil
			}
			var st stagedTransaction
			if err := json.Unmarshal(raw, &st); err != nil {
				return "A transação preparada expirou ou está inválida. Prepare novamente.", nil
			}
			postedOn, err := time.Parse("2006-01-02", st.PostedOn)
			if err != nil {
				return "Data da transação preparada é inválida. Prepare novamente.", nil
			}
			tx, err := s.transactionSvc.CreateManual(ctx, models.CreateTransactionDTO{
				UserID:      userID,
				AccountID:   st.AccountID,
				CategoryID:  st.CategoryID,
				PostedOn:    postedOn,
				Description: st.Description,
				Amount:      st.Amount,
				Direction:   st.Direction,
			})
			if err != nil {
				var dup *DuplicateError
				if asDuplicate(err, &dup) {
					s.rdb.Del(ctx, s.pendingTxKey(userID))
					return "Não gravei: já existe uma transação igual. " + dup.Error(), nil
				}
				return "", fmt.Errorf("gravar transação: %w", err)
			}
			s.rdb.Del(ctx, s.pendingTxKey(userID))
			return fmt.Sprintf("✔ Transação registrada com sucesso (id %d): %q de %s na conta %s.",
				tx.ID, st.Description, formatNumeric2(st.Amount), st.AccountName), nil
		},
	}
}
