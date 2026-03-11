package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/store"
	"strings"
)

type CreateWorkspaceDTO struct {
	UserID          int64
	Name            string
	DefaultCurrency string
}

type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, input CreateWorkspaceDTO) (store.Workspace, error)
	GetByOwnerUserID(ctx context.Context, userID int64) (store.Workspace, error)
	ExistsForUser(ctx context.Context, userID int64) (bool, error)
}

type PGWorkspaceService struct {
	db *store.Queries
}

// CreateWorkspace implements [WorkspaceService].
func (p *PGWorkspaceService) CreateWorkspace(ctx context.Context, input CreateWorkspaceDTO) (store.Workspace, error) {
	user, err := p.db.GetUserById(ctx, input.UserID)
	if err != nil {
		return store.Workspace{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Meu Workspace"
	}

	currency := strings.ToUpper(strings.TrimSpace(input.DefaultCurrency))
	if currency == "" {
		currency = "BRL"
	}

	return p.db.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		OwnerUserID:     user.ID,
		Name:            name,
		DefaultCurrency: currency,
	})
}

// ExistsForUser implements [WorkspaceService].
func (p *PGWorkspaceService) ExistsForUser(ctx context.Context, userID int64) (bool, error) {
	_, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	return false, err
}

// GetByOwnerUserID implements [WorkspaceService].
func (p *PGWorkspaceService) GetByOwnerUserID(ctx context.Context, userID int64) (store.Workspace, error) {
	return p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
}

func NewPGWorkspaceService(db *store.Queries) WorkspaceService {
	return &PGWorkspaceService{
		db,
	}
}
