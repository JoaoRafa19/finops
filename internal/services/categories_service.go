package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/store"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
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
	GetCategoryByID(ctx context.Context, userID, categoryID int64) (store.Category, error)
	CreateCategory(ctx context.Context, dto CreateCategoryDTO) (*store.Category, error)
	UpdateCategoryName(ctx context.Context, userID, categoryID int64, name string) error
	DeleteCategory(ctx context.Context, userID, categoryID int64) error
	GetUncategorized(ctx context.Context, userID int64) (int32, error)
}

type PGCategoryService struct {
	db *store.Queries
}

// GetUncategorized implements [CategoryService].
func (p *PGCategoryService) GetUncategorized(ctx context.Context, UserId int64) (int32, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, UserId)
	if err != nil {
		return -1, err
	}

	uncat, err := p.db.CountUnclassifiedTransactions(ctx, ws.ID)
	return uncat, err

}

// GetCategoryByID implements [CategoryService].
func (p *PGCategoryService) GetCategoryByID(ctx context.Context, userID, categoryID int64) (store.Category, error) {
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return store.Category{}, fmt.Errorf("error geting workspace %w", err)
	}
	return p.db.GetCategoryByWorkspaceAndID(ctx, store.GetCategoryByWorkspaceAndIDParams{
		WorkspaceID: workspace.ID,
		ID:          categoryID,
	})
}

// UpdateCategoryName implements [CategoryService].
func (p *PGCategoryService) UpdateCategoryName(ctx context.Context, userID, categoryID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("Category name required")
	}
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("error geting workspace %w", err)
	}
	err = p.db.UpdateCategoryName(ctx, store.UpdateCategoryNameParams{
		WorkspaceID: workspace.ID,
		ID:          categoryID,
		Name:        name,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errors.New("category already exists")
		}
		return fmt.Errorf("error updating category, %w", err)
	}
	return nil
}

// DeleteCategory implements [CategoryService].
func (p *PGCategoryService) DeleteCategory(ctx context.Context, userID int64, categoryID int64) error {
	workspace, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("error geting workspace %w", err)
	}

	err = p.db.DeleteCategory(ctx, store.DeleteCategoryParams{
		WorkspaceID: workspace.ID,
		ID:          categoryID,
	})

	return err
}

// CreateCategory implements [CategoryService].
func (p *PGCategoryService) CreateCategory(ctx context.Context, dto CreateCategoryDTO) (*store.Category, error) {
	name := strings.TrimSpace(dto.Name)
	if name == "" {
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
		Name:        name,
		ParentID:    sql.NullInt64{},
		Kind:        string(dto.Kind),
	})
	if err == nil {
		return &cat, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		existingCategory, lookupErr := p.db.GetCategoryByWorkspaceAndName(ctx, store.GetCategoryByWorkspaceAndNameParams{
			WorkspaceID: workspace.ID,
			Name:        name,
		})
		if lookupErr != nil {
			return nil, fmt.Errorf("error loading duplicated category, %w", lookupErr)
		}

		if existingCategory.Archived {
			if unarchiveErr := p.db.UnarchiveCategory(ctx, store.UnarchiveCategoryParams{
				WorkspaceID: workspace.ID,
				Name:        existingCategory.Name,
			}); unarchiveErr != nil {
				return nil, fmt.Errorf("error unarchiving category, %w", unarchiveErr)
			}

			existingCategory.Archived = false
			return &existingCategory, nil
		}

		return nil, errors.New("category already exists")
	}

	return nil, fmt.Errorf("error creating category, %w", err)
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
