package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/observability"
	"finops/internal/store"
	"strings"
	"time"
)

type CreateAccountDTO struct {
	UserID         int64
	Name           string
	Type           string
	Currency       string
	OpeningBalance float64
	OpeningDate    *time.Time
}

type AccountSummary struct {
	ID             int64
	Name           string
	Type           string
	Currency       string
	OpeningBalance float64
	CurrentBalance float64
}

type UpdateAccountDTO struct {
	UserID         int64
	AccountID      int64
	Name           string
	Type           string
	Currency       string
	OpeningBalance float64
	OpeningDate    *time.Time
}

type AccountService interface {
	ListByUser(ctx context.Context, userID int64) ([]store.Account, error)
	ListSummariesByUser(ctx context.Context, userID int64) ([]AccountSummary, error)
	GetByID(ctx context.Context, userID, accountID int64) (store.Account, error)
	Create(ctx context.Context, createDto CreateAccountDTO) (store.Account, error)
	Update(ctx context.Context, updateDto UpdateAccountDTO) (store.Account, error)
}

type PGAccountService struct {
	db *store.Queries
}

// Create implements [AccountService].
func (p *PGAccountService) Create(ctx context.Context, createDto CreateAccountDTO) (store.Account, error) {
	logger := observability.Logger(ctx)

	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, createDto.UserID)
	if err != nil {
		logger.Error("account_service_workspace_lookup_failed", "user_id", createDto.UserID, "error", err)
		return store.Account{}, err
	}

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

	openingDate := sql.NullTime{}

	if createDto.OpeningDate != nil {
		openingDate = sql.NullTime{
			Time:  createDto.OpeningDate.UTC(),
			Valid: true,
		}
	}

	account, err := p.db.CreateAccount(ctx, store.CreateAccountParams{
		WorkspaceID:    workspace.ID,
		Name:           name,
		Type:           accoutType,
		Currency:       currency,
		OpeningBalance: createDto.OpeningBalance,
		OpeningDate:    openingDate,
	})

	if err != nil {
		logger.Error("account_service_create_failed", "user_id", createDto.UserID, "workspace_id", workspace.ID, "error", err)
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
			OpeningBalance: row.OpeningBalance,
			CurrentBalance: row.CurrentBalance,
		})
	}

	logger.Debug("account_service_list_summaries_succeeded", "user_id", userID, "workspace_id", workspace.ID, "accounts_count", len(items))
	return items, nil
}

// Update implements [AccountService].
func (p *PGAccountService) Update(ctx context.Context, updateDto UpdateAccountDTO) (store.Account, error) {
	logger := observability.Logger(ctx)
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, updateDto.UserID)
	if err != nil {
		logger.Error("account_service_update_workspace_lookup_failed", "user_id", updateDto.UserID, "error", err)
		return store.Account{}, err
	}

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

	openingDate := sql.NullTime{}
	if updateDto.OpeningDate != nil {
		openingDate = sql.NullTime{
			Time:  updateDto.OpeningDate.UTC(),
			Valid: true,
		}
	}

	account, err := p.db.UpdateAccount(ctx, store.UpdateAccountParams{
		WorkspaceID:    workspace.ID,
		ID:             updateDto.AccountID,
		Name:           name,
		Type:           accountType,
		Currency:       currency,
		OpeningBalance: updateDto.OpeningBalance,
		OpeningDate:    openingDate,
	})
	if err != nil {
		logger.Error("account_service_update_failed", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", updateDto.AccountID, "error", err)
		return store.Account{}, err
	}

	logger.Info("account_service_update_succeeded", "user_id", updateDto.UserID, "workspace_id", workspace.ID, "account_id", account.ID)
	return account, nil
}

func NewPGAccountService(q *store.Queries) AccountService {
	return &PGAccountService{
		db: q,
	}
}
