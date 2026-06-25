package service

import (
	"context"
	"database/sql"
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
	Transaction        UnclassifiedTransaction
	SuggestedCatName   string
	SuggestedCatID     int64 // 0 se IA não conseguiu mapear para uma categoria existente
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
	db    *store.Queries
	aiSvc AIService
}

func NewPGClassificationService(q *store.Queries, ai AIService) ClassificationService {
	return &PGClassificationService{db: q, aiSvc: ai}
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

	rows, err := s.db.ListUnclassifiedTransactions(ctx, store.ListUnclassifiedTransactionsParams{
		WorkspaceID: ws.ID,
		Limit:       200,
	})
	if err != nil {
		return "", err
	}

	var description, direction string
	for _, r := range rows {
		if r.ID == txID {
			description = r.Description
			direction = r.Direction
			break
		}
	}
	if description == "" {
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

	// Se não há categorias cadastradas, IA não tem base para sugerir
	if len(catNames) == 0 {
		return "Sem categoria", nil
	}

	examples := s.loadExamples(ctx, ws.ID)
	suggestion, err := s.aiSvc.SuggestCategory(ctx, description, direction, catNames, examples)
	if err != nil {
		logger.Warn("ai_suggest_category_failed", "tx_id", txID, "error", err)
		return "Sem categoria", nil
	}
	return suggestion, nil
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

	examples := s.loadExamples(ctx, ws.ID)

	// Monta a lista de inputs para a chamada bulk
	bulkInputs := make([]BulkClassifyInput, 0, len(txRows))
	txByID := make(map[int64]store.ListUnclassifiedTransactionsRow, len(txRows))
	for _, row := range txRows {
		bulkInputs = append(bulkInputs, BulkClassifyInput{
			ID:          row.ID,
			Description: row.Description,
			Direction:   row.Direction,
		})
		txByID[row.ID] = row
	}

	// Uma única chamada à IA para todas as transações
	aiResults := map[int64]string{}
	if len(catNames) > 0 {
		var err error
		aiResults, err = s.aiSvc.SuggestCategoryBulk(ctx, bulkInputs, catNames, examples)
		if err != nil {
			logger.Warn("bulk_suggest_ai_failed", "error", err)
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

func (s *PGClassificationService) loadExamples(ctx context.Context, workspaceID int64) []ClassificationExample {
	rows, err := s.db.GetTransactionDescriptionCategory(ctx, workspaceID)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool, len(rows))
	result := make([]ClassificationExample, 0, len(rows))
	for _, r := range rows {
		if r.Category == "" {
			continue
		}
		key := strings.ToLower(r.Description)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ClassificationExample{
			Description: r.Description,
			Category:    r.Category,
		})
	}
	return result
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