package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/store"
	"fmt"
	"strings"
)

type CreateCategoryDTO struct {
	Kind   CategoryKind
	UserID int64
	Name   string
}

type CategoryKind string

const (
	EXPENSE  CategoryKind = "expense"
	INCOME   CategoryKind = "income"
	TRANSFER CategoryKind = "transfer"
)

type CategoryService interface {
	GetCategories(ctx context.Context, userid int64) ([]store.Category, error)
	CreateCategory(ctx context.Context, dto CreateCategoryDTO) (*store.Category, error)
}

type PGCategoryService struct {
	db *store.Queries
}

// CreateCategory implements [CategoryService].
func (p *PGCategoryService) CreateCategory(ctx context.Context, dto CreateCategoryDTO) (*store.Category, error) {
	if strings.TrimSpace(dto.Name) == "" {
		return nil, errors.New("Category name required")
	}

	switch dto.Kind {
	case EXPENSE, INCOME, TRANSFER:
	default:
		return nil, errors.New("unexpected category kind")
	}

	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, dto.UserID)
	if err != nil {
		return nil, fmt.Errorf("error geting workspace %w", err)
	}

	cat, err := p.db.CreateCategory(ctx, store.CreateCategoryParams{
		WorkspaceID: workspace.ID,
		Name:        strings.TrimSpace(dto.Name),
		ParentID:    sql.NullInt64{},
		Kind:        string(dto.Kind),
	})

	if err != nil {
		return nil, fmt.Errorf("error creating category, %w", err)
	}

	return &cat, nil
}

// GetCategories implements [CategoryService].
func (p *PGCategoryService) GetCategories(ctx context.Context, userid int64) ([]store.Category, error) {
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userid)
	if err != nil {
		return nil, fmt.Errorf("error geting workspace %w", err)
	}

	cats, err := p.db.GetCategories(ctx, workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("error geting categories %w", err)
	}

	return cats, nil
}

func NewPGCategoryService(db *store.Queries) CategoryService {
	return &PGCategoryService{
		db: db,
	}
}
