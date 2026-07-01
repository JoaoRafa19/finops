package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/observability"
	"finops/internal/store"
	"math"
	"strings"
	"time"
)

type CreateAccountDTO struct {
	UserID         int64
	Name           string
	Type           string
	Currency       string
	CurrentBalance float64
}

type AccountSummary struct {
	ID             int64
	Name           string
	Type           string
	Currency       string
	CurrentBalance float64
}

type UpdateAccountDTO struct {
	UserID         int64
	AccountID      int64
	Name           string
	Type           string
	Currency       string
	CurrentBalance float64
}

type AccountService interface {
	ListByUser(ctx context.Context, userID int64) ([]store.Account, error)
	ListSummariesByUser(ctx context.Context, userID int64) ([]AccountSummary, error)
	GetByID(ctx context.Context, userID, accountID int64) (store.Account, error)
	Create(ctx context.Context, createDto CreateAccountDTO) (store.Account, error)
	Update(ctx context.Context, updateDto UpdateAccountDTO) (store.Account, error)
	Delete(ctx context.Context, userID, accountID int64) error
}

type PGAccountService struct {
	sqlDB *sql.DB
	db    *store.Queries
}

// Create implements [AccountService].
func (p *PGAccountService) Create(ctx context.Context, createDto CreateAccountDTO) (store.Account, error) {
	logger := observability.Logger(ctx)

	name := strings.TrimSpace(createDto.Name)
	accoutType := strings.TrimSpace(createDto.Type)
	currency := strings.ToUpper(strings.TrimSpace(createDto.Currency))

	if name == "" {
		logger.Warn("account_service_invalid_name", "user_id", createDto.UserID)
		return store.Account{}, errors.New("account name is required")
	}
	if accoutType == "" {
		logger.Warn("account_service_invalid_type", "user_id", createDto.UserID)
		return store.Account{}, errors.New("account type is required")
	}

	if currency == "" {
		logger.Warn("account_service_invalid_currency", "user_id", createDto.UserID)
		return store.Account{}, errors.New("currency is required")
	}

	tx, err := p.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("account_service_create_begin_tx_failed", "user_id", createDto.UserID, "error", err)
		return store.Account{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := p.db.WithTx(tx)

	workspace, err := queries.GetWorkSpaceByOwnerUserID(ctx, createDto.UserID)
	if err != nil {
		logger.Error("account_service_workspace_lookup_failed", "user_id", createDto.UserID, "error", err)
		return store.Account{}, err
	}

	account, err := queries.CreateAccount(ctx, store.CreateAccountParams{
		WorkspaceID: workspace.ID,
		Name:        name,
		Type:        accoutType,
		Currency:    currency,
	})
	if err != nil {
		logger.Error("account_service_create_failed", "user_id", createDto.UserID, "workspace_id", workspace.ID, "error", err)
		return store.Account{}, err
	}

	if err := createBalanceAdjustment(ctx, queries, workspace.ID, account, createDto.CurrentBalance); err != nil {
		logger.Error("account_service_create_adjustment_failed", "user_id", createDto.UserID, "workspace_id", workspace.ID, "account_id", account.ID, "error", err)
		return store.Account{}, err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("account_service_create_commit_failed", "user_id", createDto.UserID, "workspace_id", workspace.ID, "error", err)
		return store.Account{}, err
	}

	logger.Info("account_service_create_succeeded", "user_id", createDto.UserID, "workspace_id", workspace.ID, "account_id", account.ID, "account_type", account.Type, "currency", account.Currency)
	return account, nil
}

// GetByID implements [AccountService].
func (p *PGAccountService) GetByID(ctx context.Context, userID, accountID int64) (store.Account, error) {
	logger := observability.Logger(ctx)
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		logger.Error("account_service_get_workspace_lookup_failed", "user_id", userID, "error", err)
		return store.Account{}, err
	}

	account, err := p.db.GetAccountByWorkspaceAndID(ctx, store.GetAccountByWorkspaceAndIDParams{
		WorkspaceID: workspace.ID,
		ID:          accountID,
	})
	if err != nil {
		logger.Error("account_service_get_failed", "user_id", userID, "workspace_id", workspace.ID, "account_id", accountID, "error", err)
		return store.Account{}, err
	}

	logger.Debug("account_service_get_succeeded", "user_id", userID, "workspace_id", workspace.ID, "account_id", accountID)
	return account, nil
}

// ListByUser implements [AccountService].
func (p *PGAccountService) ListByUser(ctx context.Context, userID int64) ([]store.Account, error) {
	logger := observability.Logger(ctx)
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		logger.Error("account_service_list_workspace_lookup_failed", "user_id", userID, "error", err)
		return nil, err
	}

	accounts, err := p.db.ListAccountsByWorkspace(ctx, workspace.ID)
	if err != nil {
		logger.Error("account_service_list_failed", "user_id", userID, "workspace_id", workspace.ID, "error", err)
		return nil, err
	}

	logger.Debug("account_service_list_succeeded", "user_id", userID, "workspace_id", workspace.ID, "accounts_count", len(accounts))
	return accounts, nil
}

// ListSummariesByUser implements [AccountService].
func (p *PGAccountService) ListSummariesByUser(ctx context.Context, userID int64) ([]AccountSummary, error) {
	logger := observability.Logger(ctx)
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		logger.Error("account_service_list_summaries_workspace_lookup_failed", "user_id", userID, "error", err)
		return nil, err
	}

	rows, err := p.db.ListAccountSummariesByWorkspace(ctx, workspace.ID)
	if err != nil {
		logger.Error("account_service_list_summaries_failed", "user_id", userID, "workspace_id", workspace.ID, "error", err)
		return nil, err
	}

	items := make([]AccountSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, AccountSummary{
			ID:             row.ID,
			Name:           row.Name,
			Type:           row.Type,
			Currency:       row.Currency,
			CurrentBalance: row.CurrentBalance,
		})
	}

	logger.Debug("account_service_list_summaries_succeeded", "user_id", userID, "workspace_id", workspace.ID, "accounts_count", len(items))
	return items, nil
}

// Update implements [AccountService].
func (p *PGAccountService) Update(ctx context.Context, updateDto UpdateAccountDTO) (store.Account, error) {
	logger := observability.Logger(ctx)

	name := strings.TrimSpace(updateDto.Name)
	accountType := strings.TrimSpace(updateDto.Type)
	currency := strings.ToUpper(strings.TrimSpace(updateDto.Currency))

	if name == "" {
		logger.Warn("account_service_update_invalid_name", "user_id", updateDto.UserID, "account_id", updateDto.AccountID)
		return store.Account{}, errors.New("account name is required")
	}
	if accountType == "" {
		logger.Warn("account_service_update_invalid_type", "user_id", updateDto.UserID, "account_id", updateDto.AccountID)
		return store.Account{}, errors.New("account type is required")
	}
	if currency == "" {
		logger.Warn("account_service_update_invalid_currency", "user_id", updateDto.UserID, "account_id", updateDto.AccountID)
		return store.Account{}, errors.New("currency is required")
	}

	tx, err := p.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("account_service_update_begin_tx_failed", "user_id", updateDto.UserID, "account_id", updateDto.AccountID, "error", err)
		return store.Account{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := p.db.WithTx(tx)

	workspace, err := queries.GetWorkSpaceByOwnerUserID(ctx, updateDto.UserID)
	if err != nil {
		logger.Error("account_service_update_workspace_lookup_failed", "user_id", updateDto.UserID, "error", err)
		return store.Account{}, err
	}

	currentBalance, err := currentBalanceForAccount(ctx, queries, workspace.ID, updateDto.AccountID)
	if err != nil {
		logger.Error("account_service_update_current_balance_lookup_failed", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", updateDto.AccountID, "error", err)
		return store.Account{}, err
	}

	account, err := queries.UpdateAccount(ctx, store.UpdateAccountParams{
		WorkspaceID: workspace.ID,
		ID:          updateDto.AccountID,
		Name:        name,
		Type:        accountType,
		Currency:    currency,
	})
	if err != nil {
		logger.Error("account_service_update_failed", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", updateDto.AccountID, "error", err)
		return store.Account{}, err
	}

	if err := createBalanceAdjustment(ctx, queries, workspace.ID, account, updateDto.CurrentBalance-currentBalance); err != nil {
		logger.Error("account_service_update_adjustment_failed", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", updateDto.AccountID, "error", err)
		return store.Account{}, err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("account_service_update_commit_failed", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", updateDto.AccountID, "error", err)
		return store.Account{}, err
	}

	logger.Info("account_service_update_succeeded", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", account.ID)
	return account, nil
}

// Delete implements [AccountService]. Removes the account and all its transactions permanently.
func (p *PGAccountService) Delete(ctx context.Context, userID, accountID int64) error {
	logger := observability.Logger(ctx)

	tx, err := p.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	queries := p.db.WithTx(tx)

	workspace, err := queries.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return err
	}

	if _, err := queries.GetAccountByWorkspaceAndID(ctx, store.GetAccountByWorkspaceAndIDParams{
		WorkspaceID: workspace.ID,
		ID:          accountID,
	}); err != nil {
		return err
	}

	if err := queries.DeleteTransactionsByAccount(ctx, store.DeleteTransactionsByAccountParams{
		WorkspaceID: workspace.ID,
		AccountID:   accountID,
	}); err != nil {
		return err
	}

	if err := queries.HardDeleteAccount(ctx, store.HardDeleteAccountParams{
		WorkspaceID: workspace.ID,
		ID:          accountID,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Info("account_service_delete_succeeded", "user_id", userID, "workspace_id", workspace.ID, "account_id", accountID)
	return nil
}

func createBalanceAdjustment(ctx context.Context, queries *store.Queries, workspaceID int64, account store.Account, delta float64) error {
	if math.Abs(delta) < 0.00005 {
		return nil
	}

	direction := "credit"
	if delta < 0 {
		direction = "debit"
	}

	_, err := queries.CreateTransaction(ctx, store.CreateTransactionParams{
		WorkspaceID:     workspaceID,
		AccountID:       account.ID,
		CategoryID:      sql.NullInt64{},
		PostedOn:        time.Now().UTC(),
		Description:     "Ajuste de saldo",
		Amount:          formatNumeric(math.Abs(delta)),
		Direction:       direction,
		Currency:        account.Currency,
		TransferGroupID: sql.NullInt64{},
		ExternalFitid:   sql.NullString{},
		Source:          "adjustment",
	})
	return err
}

func currentBalanceForAccount(ctx context.Context, queries *store.Queries, workspaceID, accountID int64) (float64, error) {
	rows, err := queries.ListAccountSummariesByWorkspace(ctx, workspaceID)
	if err != nil {
		return 0, err
	}

	for _, row := range rows {
		if row.ID == accountID {
			return row.CurrentBalance, nil
		}
	}

	return 0, sql.ErrNoRows
}

func NewPGAccountService(sqlDB *sql.DB, q *store.Queries) AccountService {
	return &PGAccountService{
		sqlDB: sqlDB,
		db:    q,
	}
}
