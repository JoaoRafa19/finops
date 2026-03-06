package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/store"
	"strings"
	"time"
)

type CreateAccountDTO struct {
	UserID         int64
	Name           string
	Type           string
	Currency       string
	OpeningBalance string
	OpeningDate    *time.Time
}

type AccountService interface {
	ListByUser(ctx context.Context, userID int64) ([]store.Account, error)
	Create(ctx context.Context, createDto CreateAccountDTO) (store.Account, error)
}

type PGAccountService struct {
	db *store.Queries
}

// Create implements [AccountService].
func (p *PGAccountService) Create(ctx context.Context, createDto CreateAccountDTO) (store.Account, error) {

	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, createDto.UserID)
	if err != nil {
		return store.Account{}, err
	}

	name := strings.TrimSpace(createDto.Name)
	accoutType := strings.TrimSpace(createDto.Type)
	currency := strings.ToUpper(strings.TrimSpace(createDto.Currency))

	if name == "" {
		return store.Account{}, errors.New("account name is required")
	}
	if accoutType == "" {
		return store.Account{}, errors.New("account type is required")
	}

	if currency == "" {
		return store.Account{}, errors.New("currency is required")
	}

	openingDate := sql.NullTime{}

	if createDto.OpeningDate != nil {
		openingDate = sql.NullTime{
			Time:  createDto.OpeningDate.UTC(),
			Valid: true,
		}
	}

	return p.db.CreateAccount(ctx, store.CreateAccountParams{
		WorkspaceID:    workspace.ID,
		Name:           name,
		Type:           accoutType,
		Currency:       currency,
		OpeningBalance: createDto.OpeningBalance,
		OpeningDate:    openingDate,
	})
}

// ListByUser implements [AccountService].
func (p *PGAccountService) ListByUser(ctx context.Context, userID int64) ([]store.Account, error) {
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return p.db.ListAccountsByWorkspace(ctx, workspace.ID)
}

func NewPGAccountService(q *store.Queries) AccountService {
	return &PGAccountService{
		db: q,
	}
}
