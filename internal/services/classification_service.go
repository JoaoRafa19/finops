package service

import (
	"context"
	"database/sql"
	"fmt"
	"finops/internal/observability"
	"finops/internal/store"
	"strconv"
	"strings"
	"time"
)

type UnclassifiedTransaction struct {
	ID          int64
	AccountName string
	PostedOn    time.Time
	Description string
	Amount      float64
	Direction   string
	Currency    string
}

type CategoryResult struct {
	ID   int64
	Name string
	Kind string
}

type BulkSuggestion struct {
	Transaction      UnclassifiedTransaction
	SuggestedCatName string
	SuggestedCatID   int64
}

type ClassificationService interface {
	ListUnclassified(ctx context.Context, userID int64, limit int32) ([]UnclassifiedTransaction, error)
	CountUnclassified(ctx context.Context, userID int64) (int, error)
	SuggestCategory(ctx context.Context, userID int64, txID int64) (string, error)
	Classify(ctx context.Context, userID, txID, categoryID int64, description string) (int64, error)
	SearchCategories(ctx context.Context, userID int64, query string) ([]CategoryResult, error)
	BulkSuggest(ctx context.Context, userID int64, limit int32) ([]BulkSuggestion, []store.Category, error)
	BulkConfirm(ctx context.Context, userID int64, classifications map[int64]int64) (int, error)
}

type PGClassificationService struct {
	db       *store.Queries
	rawDB    *sql.DB
	aiSvc    AIService
	embSvc   EmbeddingService
}

func NewPGClassificationService(q *store.Queries, rawDB *sql.DB, ai AIService, emb EmbeddingService) ClassificationService {
	return &PGClassificationService{db: q, rawDB: rawDB, aiSvc: ai, embSvc: emb}
}

func (s *PGClassificationService) ListUnclassified(ctx context.Context, userID int64, limit int32) ([]UnclassifiedTransaction, error) {
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.ListUnclassifiedTransactions(ctx, store.ListUnclassifiedTransactionsParams{
		WorkspaceID: ws.ID,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}

	result := make([]UnclassifiedTransaction, 0, len(rows))
	for _, r := range rows {
		amt, _ := strconv.ParseFloat(r.Amount, 64)
		result = append(result, UnclassifiedTransaction{
			ID:          r.ID,
			AccountName: r.AccountName,
			PostedOn:    r.PostedOn,
			Description: r.Description,
			Amount:      amt,
			Direction:   r.Direction,
			Currency:    r.Currency,
		})
	}
	return result, nil
}

func (s *PGClassificationService) CountUnclassified(ctx context.Context, userID int64) (int, error) {
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	n, err := s.db.CountUnclassifiedTransactions(ctx, ws.ID)
	return int(n), err
}

func (s *PGClassificationService) SuggestCategory(ctx context.Context, userID int64, txID int64) (string, error) {
	logger := observability.Logger(ctx)

	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return "", err
	}

	description, direction, err := s.getTransactionByID(ctx, ws.ID, txID)
	if err != nil || description == "" {
		return "Sem categoria", nil
	}

	cats, err := s.db.GetCategories(ctx, ws.ID)
	if err != nil {
		return "", err
	}

	catNames := make([]string, 0, len(cats))
	for _, c := range cats {
		if c.Kind != "transfer" {
			catNames = append(catNames, c.Name)
		}
	}
	if len(catNames) == 0 {
		return "Sem categoria", nil
	}

	dirLabel := "despesa"
	if direction == "credit" {
		dirLabel = "receita"
	}

	system := `Você é um classificador de transações financeiras. Tem acesso à ferramenta search_similar_classifications para consultar como transações semelhantes foram classificadas anteriormente. Use-a para embasar sua decisão. Responda APENAS com o nome exato de uma das categorias disponíveis. Se nenhuma for adequada, responda: Sem categoria`

	user := fmt.Sprintf("Classifique esta transação:\nDescrição: %s\nTipo: %s\nCategorias disponíveis: %s",
		description, dirLabel, strings.Join(catNames, ", "))

	tools := ClassificationTools(s.embSvc, s.rawDB, s.db, userID)
	suggestion, err := s.aiSvc.RunAgentLoop(ctx, system, user, tools, 4)
	if err != nil {
		logger.Warn("ai_suggest_category_failed", "tx_id", txID, "error", err)
		return "Sem categoria", nil
	}
	return strings.Trim(suggestion, `"'`), nil
}

func (s *PGClassificationService) Classify(ctx context.Context, userID, txID, categoryID int64, description string) (int64, error) {
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	nullCatID := sql.NullInt64{Int64: categoryID, Valid: true}

	if err := s.db.ClassifyTransaction(ctx, store.ClassifyTransactionParams{
		ID:          txID,
		CategoryID:  nullCatID,
		WorkspaceID: ws.ID,
	}); err != nil {
		return 0, err
	}

	if description == "" {
		return 0, nil
	}

	keyword := normalizeKeyword(description)

	_ = s.db.UpsertClassificationRule(ctx, store.UpsertClassificationRuleParams{
		WorkspaceID: ws.ID,
		Keyword:     keyword,
		CategoryID:  categoryID,
	})

	pattern := "%" + escapeLike(keyword) + "%"
	affected, err := s.db.ClassifyTransactionsByKeyword(ctx, store.ClassifyTransactionsByKeywordParams{
		WorkspaceID: ws.ID,
		Lower:       pattern,
		CategoryID:  nullCatID,
	})
	if err != nil {
		return 0, err
	}

	// Guarda embedding em background para buscas futuras
	if catName := s.resolveCategoryName(ctx, ws.ID, categoryID); catName != "" {
		go s.storeEmbedding(context.WithoutCancel(ctx), ws.ID, description, catName)
	}

	return affected, nil
}

func (s *PGClassificationService) SearchCategories(ctx context.Context, userID int64, query string) ([]CategoryResult, error) {
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	pattern := "%" + strings.TrimSpace(query) + "%"
	rows, err := s.db.SearchCategories(ctx, store.SearchCategoriesParams{
		WorkspaceID: ws.ID,
		Lower:       pattern,
	})
	if err != nil {
		return nil, err
	}

	result := make([]CategoryResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, CategoryResult{ID: r.ID, Name: r.Name, Kind: r.Kind})
	}
	return result, nil
}

func (s *PGClassificationService) BulkSuggest(ctx context.Context, userID int64, limit int32) ([]BulkSuggestion, []store.Category, error) {
	logger := observability.Logger(ctx)

	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	if limit <= 0 || limit > 20 {
		limit = 10
	}

	txRows, err := s.db.ListUnclassifiedTransactions(ctx, store.ListUnclassifiedTransactionsParams{
		WorkspaceID: ws.ID,
		Limit:       limit,
	})
	if err != nil {
		return nil, nil, err
	}

	cats, err := s.db.GetCategories(ctx, ws.ID)
	if err != nil {
		return nil, nil, err
	}

	catNames := make([]string, 0, len(cats))
	catByName := make(map[string]store.Category, len(cats))
	for _, c := range cats {
		if c.Kind != "transfer" {
			catNames = append(catNames, c.Name)
			catByName[strings.ToLower(c.Name)] = c
		}
	}

	txByID := make(map[int64]store.ListUnclassifiedTransactionsRow, len(txRows))
	var txLines strings.Builder
	for _, row := range txRows {
		dirLabel := "despesa"
		if row.Direction == "credit" {
			dirLabel = "receita"
		}
		fmt.Fprintf(&txLines, "- ID:%d | %s | %s\n", row.ID, row.Description, dirLabel)
		txByID[row.ID] = row
	}

	aiResults := map[int64]string{}
	if len(catNames) > 0 && len(txRows) > 0 {
		system := `Você é um classificador de transações financeiras. Tem acesso à ferramenta search_similar_classifications para consultar como transações semelhantes foram classificadas. Use-a para embasar suas decisões. Responda APENAS com um array JSON no formato: [{"id": 123, "category": "NomeDaCategoria"}, ...]`

		user := fmt.Sprintf("Classifique as transações abaixo. Use apenas as categorias listadas.\n\nTransações:\n%s\nCategorias disponíveis: %s\n\nResponda APENAS com o array JSON.",
			txLines.String(), strings.Join(catNames, ", "))

		tools := ClassificationTools(s.embSvc, s.rawDB, s.db, userID)
		raw, err := s.aiSvc.RunAgentLoop(ctx, system, user, tools, 6)
		if err != nil {
			logger.Warn("bulk_suggest_ai_failed", "error", err)
		} else {
			aiResults = parseBulkResponse(raw)
		}
	}

	suggestions := make([]BulkSuggestion, 0, len(txRows))
	for _, row := range txRows {
		amt, _ := strconv.ParseFloat(row.Amount, 64)
		tx := UnclassifiedTransaction{
			ID:          row.ID,
			AccountName: row.AccountName,
			PostedOn:    row.PostedOn,
			Description: row.Description,
			Amount:      amt,
			Direction:   row.Direction,
			Currency:    row.Currency,
		}

		suggestion := BulkSuggestion{Transaction: tx}
		if name, ok := aiResults[row.ID]; ok && name != "" && name != "Sem categoria" {
			suggestion.SuggestedCatName = name
			if cat, ok := catByName[strings.ToLower(strings.TrimSpace(name))]; ok {
				suggestion.SuggestedCatID = cat.ID
			}
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, cats, nil
}

func (s *PGClassificationService) BulkConfirm(ctx context.Context, userID int64, classifications map[int64]int64) (int, error) {
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	confirmed := 0
	for txID, categoryID := range classifications {
		if categoryID == 0 {
			continue
		}
		nullCatID := sql.NullInt64{Int64: categoryID, Valid: true}
		if err := s.db.ClassifyTransaction(ctx, store.ClassifyTransactionParams{
			ID:          txID,
			CategoryID:  nullCatID,
			WorkspaceID: ws.ID,
		}); err != nil {
			continue
		}
		confirmed++
	}
	return confirmed, nil
}

// --- helpers ---

func (s *PGClassificationService) getTransactionByID(ctx context.Context, workspaceID, txID int64) (description, direction string, err error) {
	row := s.rawDB.QueryRowContext(ctx,
		`SELECT description, direction FROM transactions WHERE id = $1 AND workspace_id = $2`,
		txID, workspaceID,
	)
	err = row.Scan(&description, &direction)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return description, direction, err
}

func (s *PGClassificationService) resolveCategoryName(ctx context.Context, workspaceID, categoryID int64) string {
	var name string
	_ = s.rawDB.QueryRowContext(ctx,
		`SELECT name FROM categories WHERE id = $1 AND workspace_id = $2`,
		categoryID, workspaceID,
	).Scan(&name)
	return name
}

func (s *PGClassificationService) storeEmbedding(ctx context.Context, workspaceID int64, description, category string) {
	emb, err := s.embSvc.Embed(ctx, description)
	if err != nil {
		return
	}
	_ = store.UpsertClassificationEmbedding(ctx, s.rawDB, workspaceID, description, category, emb)
}

func normalizeKeyword(desc string) string {
	return strings.ToLower(strings.TrimSpace(desc))
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}