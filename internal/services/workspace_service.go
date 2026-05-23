package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/observability"
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
	logger := observability.Logger(ctx)
	user, err := p.db.GetUserById(ctx, input.UserID)
	if err != nil {
		logger.Error("workspace_create_load_user_failed", "user_id", input.UserID, "error", err)
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

	workspace, err := p.db.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		OwnerUserID:     user.ID,
		Name:            name,
		DefaultCurrency: currency,
	})
	if err != nil {
		logger.Error("workspace_create_failed", "user_id", input.UserID, "error", err)
		return store.Workspace{}, err
	}

	logger.Info("workspace_create_succeeded", "user_id", input.UserID, "workspace_id", workspace.ID, "default_currency", workspace.DefaultCurrency)
	return workspace, nil
}

// ExistsForUser implements [WorkspaceService].
func (p *PGWorkspaceService) ExistsForUser(ctx context.Context, userID int64) (bool, error) {
	logger := observability.Logger(ctx)
	_, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err == nil {
		logger.Debug("workspace_exists_for_user", "user_id", userID, "exists", true)
		return true, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug("workspace_exists_for_user", "user_id", userID, "exists", false)
		return false, nil
	}

	logger.Error("workspace_exists_check_failed", "user_id", userID, "error", err)
	return false, err
}

// GetByOwnerUserID implements [WorkspaceService].
func (p *PGWorkspaceService) GetByOwnerUserID(ctx context.Context, userID int64) (store.Workspace, error) {
	logger := observability.Logger(ctx)
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		logger.Error("workspace_get_by_owner_failed", "user_id", userID, "error", err)
		return store.Workspace{}, err
	}

	logger.Debug("workspace_get_by_owner_succeeded", "user_id", userID, "workspace_id", workspace.ID)
	return workspace, nil
}

func NewPGWorkspaceService(db *store.Queries) WorkspaceService {
	return &PGWorkspaceService{
		db,
	}
}
