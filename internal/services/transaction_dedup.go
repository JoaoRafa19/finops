package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"finops/internal/store"
)

// dedupWindowDays: janela de datas em que duas transações do mesmo valor/conta
// são candidatas a duplicata (±3 dias cobre atraso de compensação).
const dedupWindowDays = 3

// DuplicateMatch descreve a transação já existente parecida com a que se tenta criar.
type DuplicateMatch struct {
	ID          int64
	PostedOn    time.Time
	Description string
	Amount      float64
	Direction   string
}

// DuplicateError é retornado quando a criação bate com uma transação existente
// (conta+valor+data±3d+descrição parecida). Bloqueia a inserção.
type DuplicateError struct {
	Match DuplicateMatch
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("transação duplicada: já existe %q de %s em %s",
		e.Match.Description, formatNumeric(e.Match.Amount), e.Match.PostedOn.Format("02/01/2006"))
}

// FindDuplicate procura uma transação existente equivalente (mesma conta, valor,
// data ±3 dias, direção e descrição parecida) para um usuário. Resolve o
// workspace e delega ao guard interno. Usado pelo chat antes de gravar.
func (p *PGTransactionService) FindDuplicate(ctx context.Context, userID, accountID int64, postedOn time.Time, amount float64, direction, description string) (DuplicateMatch, bool, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return DuplicateMatch{}, false, fmt.Errorf("get workspace: %w", err)
	}
	return p.findDuplicateInWorkspace(ctx, ws.ID, accountID, postedOn, amount, direction, description)
}

// findDuplicateInWorkspace é o guard interno (já com workspace resolvido).
func (p *PGTransactionService) findDuplicateInWorkspace(ctx context.Context, workspaceID, accountID int64, postedOn time.Time, amount float64, direction, description string) (DuplicateMatch, bool, error) {
	rows, err := p.db.FindSimilarTransactions(ctx, store.FindSimilarTransactionsParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
		Amount:      formatNumeric(amount),
		PostedOn:    postedOn.AddDate(0, 0, -dedupWindowDays),
		PostedOn_2:  postedOn.AddDate(0, 0, dedupWindowDays),
	})
	if err != nil {
		return DuplicateMatch{}, false, fmt.Errorf("find similar transactions: %w", err)
	}
	for _, r := range rows {
		if r.Direction != direction {
			continue
		}
		if !similarDescription(r.Description, description) {
			continue
		}
		return DuplicateMatch{
			ID:          r.ID,
			PostedOn:    r.PostedOn,
			Description: r.Description,
			Amount:      parseNumeric(r.Amount),
			Direction:   r.Direction,
		}, true, nil
	}
	return DuplicateMatch{}, false, nil
}

// similarDescription: duas descrições são "parecidas" se, normalizadas, forem
// iguais, uma contiver os tokens da outra, ou tiverem sobreposição de tokens
// (Jaccard) ≥ 0,5. Sem dependência externa nem pg_trgm — o filtro por
// valor+data já estreita muito os candidatos.
func similarDescription(a, b string) bool {
	ta, tb := descTokens(a), descTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return normalizeDesc(a) == normalizeDesc(b)
	}
	inter := 0
	for tok := range ta {
		if tb[tok] {
			inter++
		}
	}
	// Contido: todos os tokens do menor aparecem no maior (ex.: "uber" ⊆ "uber trip 12").
	small := len(ta)
	if len(tb) < small {
		small = len(tb)
	}
	if inter == small {
		return true
	}
	union := len(ta) + len(tb) - inter
	return float64(inter)/float64(union) >= 0.6
}

// accentFolder decompõe (NFD) e remove as marcas de acento, então "Alimentação"
// e "alimentacao" comparam iguais depois do normalizeDesc.
var accentFolder = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func foldAccents(s string) string {
	out, _, err := transform.String(accentFolder, s)
	if err != nil {
		return s
	}
	return out
}

func normalizeDesc(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(foldAccents(s))) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func descTokens(s string) map[string]bool {
	toks := map[string]bool{}
	for _, t := range strings.Fields(normalizeDesc(s)) {
		toks[t] = true
	}
	return toks
}
