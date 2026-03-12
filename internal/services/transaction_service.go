package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/observability"
	"finops/internal/store"
	"strconv"
	"strings"
	"time"
)

type CreateTransactionDTO struct {
	UserID      int64
	AccountID   int64
	PostedOn    time.Time
	Description string
	Amount      float64
	Direction   string
}

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
}

type TransactionService interface {
	CreateManual(ctx context.Context, input CreateTransactionDTO) (store.Transaction, error)
	ListRecentByUser(ctx context.Context, userID int64, limit int32) ([]TransactionListItem, error)
}

type PGTransactionService struct {
	db *store.Queries
}

func NewPGTransactionService(db *store.Queries) TransactionService {
	return &PGTransactionService{db: db}
}

func (p *PGTransactionService) CreateManual(ctx context.Context, input CreateTransactionDTO) (store.Transaction, error) {
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

	transaction, err := p.db.CreateTransaction(ctx, store.CreateTransactionParams{
		WorkspaceID:     workspace.ID,
		AccountID:       account.ID,
		CategoryID:      sql.NullInt64{},
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
		})
	}

	logger.Debug("transaction_list_recent_succeeded", "user_id", userID, "workspace_id", workspace.ID, "count", len(items))
	return items, nil
}

func formatNumeric(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}
