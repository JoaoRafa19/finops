package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/models"
	"finops/internal/observability"
	"finops/internal/store"
	"strconv"
	"strings"
	"time"
)

type TransactionListItem struct {
	ID          int64
	AccountID   int64
	AccountName string
	PostedOn    time.Time
	Description string
	Amount      float64
	Direction   string
	Currency    string
	Source      string
	Category    string
}

type TransactionService interface {
	CreateManual(ctx context.Context, input models.CreateTransactionDTO) (store.Transaction, error)
	CreateTransfer(ctx context.Context, input models.CreateTransferDTO) ([]store.Transaction, error)
	ListRecentByUser(ctx context.Context, userID int64, limit int32) ([]TransactionListItem, error)
}

type PGTransactionService struct {
	sqlDB *sql.DB
	db    *store.Queries
}

func NewPGTransactionService(sqlDB *sql.DB, db *store.Queries) TransactionService {
	return &PGTransactionService{
		sqlDB: sqlDB,
		db:    db,
	}
}

func (p *PGTransactionService) CreateManual(ctx context.Context, input models.CreateTransactionDTO) (store.Transaction, error) {
	logger := observability.Logger(ctx)

	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, input.UserID)
	if err != nil {
		logger.Error("transaction_create_workspace_lookup_failed", "user_id", input.UserID, "error", err)
		return store.Transaction{}, err
	}

	account, err := p.db.GetAccountByWorkspaceAndID(ctx, store.GetAccountByWorkspaceAndIDParams{
		WorkspaceID: workspace.ID,
		ID:          input.AccountID,
	})
	if err != nil {
		logger.Error("transaction_create_account_lookup_failed", "user_id", input.UserID, "account_id", input.AccountID, "error", err)
		return store.Transaction{}, err
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		logger.Warn("transaction_create_invalid_description", "user_id", input.UserID, "account_id", input.AccountID)
		return store.Transaction{}, errors.New("description is required")
	}

	direction := strings.TrimSpace(input.Direction)
	if direction != "credit" && direction != "debit" {
		logger.Warn("transaction_create_invalid_direction", "user_id", input.UserID, "account_id", input.AccountID, "direction", input.Direction)
		return store.Transaction{}, errors.New("direction is required")
	}

	if input.Amount <= 0 {
		logger.Warn("transaction_create_invalid_amount", "user_id", input.UserID, "account_id", input.AccountID, "amount", input.Amount)
		return store.Transaction{}, errors.New("amount must be greater than zero")
	}

	categoryID := sql.NullInt64{}
	if input.CategoryID != nil {
		category, err := p.db.GetCategoryByWorkspaceAndID(ctx, store.GetCategoryByWorkspaceAndIDParams{
			WorkspaceID: workspace.ID,
			ID:          *input.CategoryID,
		})
		if err != nil {
			logger.Error("transaction_create_category_lookup_failed", "user_id", input.UserID, "account_id", input.AccountID, "category_id", *input.CategoryID, "error", err)
			return store.Transaction{}, err
		}

		categoryID = sql.NullInt64{Int64: category.ID, Valid: true}
	}

	transaction, err := p.db.CreateTransaction(ctx, store.CreateTransactionParams{
		WorkspaceID:     workspace.ID,
		AccountID:       account.ID,
		CategoryID:      categoryID,
		PostedOn:        input.PostedOn,
		Description:     description,
		Amount:          formatNumeric(input.Amount),
		Direction:       direction,
		Currency:        account.Currency,
		TransferGroupID: sql.NullInt64{},
		ExternalFitid:   sql.NullString{},
		Source:          "manual",
	})
	if err != nil {
		logger.Error("transaction_create_failed", "user_id", input.UserID, "account_id", input.AccountID, "error", err)
		return store.Transaction{}, err
	}

	logger.Info("transaction_create_succeeded", "user_id", input.UserID, "transaction_id", transaction.ID, "account_id", input.AccountID, "direction", direction)
	return transaction, nil
}

func (p *PGTransactionService) CreateTransfer(ctx context.Context, input models.CreateTransferDTO) ([]store.Transaction, error) {
	logger := observability.Logger(ctx)

	if input.FromAccountID == input.ToAccountID {
		return nil, errors.New("source and destination accounts must be different")
	}
	if input.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	tx, err := p.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("transfer_begin_tx_failed", "user_id", input.UserID, "error", err)
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := p.db.WithTx(tx)

	workspace, err := queries.GetWorkSpaceByOwnerUserID(ctx, input.UserID)
	if err != nil {
		logger.Error("transfer_workspace_lookup_failed", "user_id", input.UserID, "error", err)
		return nil, err
	}

	fromAccount, err := queries.GetAccountByWorkspaceAndID(ctx, store.GetAccountByWorkspaceAndIDParams{
		WorkspaceID: workspace.ID,
		ID:          input.FromAccountID,
	})
	if err != nil {
		logger.Error("transfer_source_account_lookup_failed", "user_id", input.UserID, "account_id", input.FromAccountID, "error", err)
		return nil, err
	}

	toAccount, err := queries.GetAccountByWorkspaceAndID(ctx, store.GetAccountByWorkspaceAndIDParams{
		WorkspaceID: workspace.ID,
		ID:          input.ToAccountID,
	})
	if err != nil {
		logger.Error("transfer_destination_account_lookup_failed", "user_id", input.UserID, "account_id", input.ToAccountID, "error", err)
		return nil, err
	}

	if fromAccount.Currency != toAccount.Currency {
		return nil, errors.New("transfers between different currencies are not supported")
	}

	transferCategory, err := ensureTransferCategory(ctx, queries, workspace.ID)
	if err != nil {
		logger.Error("transfer_category_ensure_failed", "user_id", input.UserID, "workspace_id", workspace.ID, "error", err)
		return nil, err
	}

	groupID := time.Now().UnixNano()
	amount := formatNumeric(input.Amount)

	debitTx, err := queries.CreateTransaction(ctx, store.CreateTransactionParams{
		WorkspaceID:     workspace.ID,
		AccountID:       fromAccount.ID,
		CategoryID:      sql.NullInt64{Int64: transferCategory.ID, Valid: true},
		PostedOn:        input.PostedOn,
		Description:     "Transferencia para " + toAccount.Name,
		Amount:          amount,
		Direction:       "debit",
		Currency:        fromAccount.Currency,
		TransferGroupID: sql.NullInt64{Int64: groupID, Valid: true},
		ExternalFitid:   sql.NullString{},
		Source:          "manual",
	})
	if err != nil {
		logger.Error("transfer_create_debit_failed", "user_id", input.UserID, "error", err)
		return nil, err
	}

	creditTx, err := queries.CreateTransaction(ctx, store.CreateTransactionParams{
		WorkspaceID:     workspace.ID,
		AccountID:       toAccount.ID,
		CategoryID:      sql.NullInt64{Int64: transferCategory.ID, Valid: true},
		PostedOn:        input.PostedOn,
		Description:     "Transferencia de " + fromAccount.Name,
		Amount:          amount,
		Direction:       "credit",
		Currency:        toAccount.Currency,
		TransferGroupID: sql.NullInt64{Int64: groupID, Valid: true},
		ExternalFitid:   sql.NullString{},
		Source:          "manual",
	})
	if err != nil {
		logger.Error("transfer_create_credit_failed", "user_id", input.UserID, "error", err)
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("transfer_commit_failed", "user_id", input.UserID, "error", err)
		return nil, err
	}

	logger.Info("transfer_create_succeeded", "user_id", input.UserID, "group_id", groupID, "from_account_id", fromAccount.ID, "to_account_id", toAccount.ID)
	return []store.Transaction{debitTx, creditTx}, nil
}

func (p *PGTransactionService) ListRecentByUser(ctx context.Context, userID int64, limit int32) ([]TransactionListItem, error) {
	logger := observability.Logger(ctx)

	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		logger.Error("transaction_list_workspace_lookup_failed", "user_id", userID, "error", err)
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}

	rows, err := p.db.ListRecentTransactionsByWorkspace(ctx, store.ListRecentTransactionsByWorkspaceParams{
		WorkspaceID: workspace.ID,
		Limit:       limit,
	})
	if err != nil {
		logger.Error("transaction_list_recent_failed", "user_id", userID, "workspace_id", workspace.ID, "error", err)
		return nil, err
	}

	items := make([]TransactionListItem, 0, len(rows))
	for _, row := range rows {
		amount, parseErr := strconv.ParseFloat(row.Amount, 64)
		if parseErr != nil {
			logger.Error("transaction_list_recent_amount_parse_failed", "user_id", userID, "transaction_id", row.ID, "amount", row.Amount, "error", parseErr)
			return nil, parseErr
		}

		category := ""
		if row.CategoryID.Valid {
			categoryRow, err := p.db.GetCategoryByWorkspaceAndID(ctx, store.GetCategoryByWorkspaceAndIDParams{
				WorkspaceID: workspace.ID,
				ID:          row.CategoryID.Int64,
			})
			if err != nil {
				logger.Error("transaction_list_recent_category_lookup_failed", "user_id", userID, "transaction_id", row.ID, "category_id", row.CategoryID.Int64, "error", err)
				return nil, err
			}
			category = categoryRow.Name
		}

		items = append(items, TransactionListItem{
			ID:          row.ID,
			AccountID:   row.AccountID,
			AccountName: row.AccountName,
			PostedOn:    row.PostedOn,
			Description: row.Description,
			Amount:      amount,
			Direction:   row.Direction,
			Currency:    row.Currency,
			Source:      row.Source,
			Category:    category,
		})
	}

	logger.Debug("transaction_list_recent_succeeded", "user_id", userID, "workspace_id", workspace.ID, "count", len(items))
	return items, nil
}

func formatNumeric(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}

func ensureTransferCategory(ctx context.Context, queries *store.Queries, workspaceID int64) (store.Category, error) {
	categories, err := queries.GetCategories(ctx, workspaceID)
	if err != nil {
		return store.Category{}, err
	}

	for _, category := range categories {
		if category.Kind == "transfer" {
			return category, nil
		}
	}

	return queries.CreateCategory(ctx, store.CreateCategoryParams{
		WorkspaceID: workspaceID,
		ParentID:    sql.NullInt64{},
		Name:        "Transferencia",
		Kind:        "transfer",
	})
}
