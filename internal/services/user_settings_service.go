package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"finops/internal/store"
)

const (
	HomeModeSimple   = "simple"
	HomeModeAdvanced = "advanced"
)

func validHomeMode(mode string) bool {
	return mode == HomeModeSimple || mode == HomeModeAdvanced
}

type UserSettingsService interface {
	// GetHomeMode devolve o modo do dashboard. Sem linha (usuário anterior à
	// feature) → advanced, para não mudar a home de quem já usava o app.
	GetHomeMode(ctx context.Context, userID int64) (string, error)
	SetHomeMode(ctx context.Context, userID int64, mode string) error
}

type PGUserSettingsService struct {
	db *store.Queries
}

func NewPGUserSettingsService(db *store.Queries) UserSettingsService {
	return &PGUserSettingsService{db: db}
}

func (s *PGUserSettingsService) GetHomeMode(ctx context.Context, userID int64) (string, error) {
	row, err := s.db.GetUserSettings(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return HomeModeAdvanced, nil
	}
	if err != nil {
		return "", fmt.Errorf("get user settings: %w", err)
	}
	return row.HomeMode, nil
}

func (s *PGUserSettingsService) SetHomeMode(ctx context.Context, userID int64, mode string) error {
	if !validHomeMode(mode) {
		return fmt.Errorf("modo inválido: %s", mode)
	}
	if err := s.db.UpsertUserHomeMode(ctx, store.UpsertUserHomeModeParams{
		UserID:   userID,
		HomeMode: mode,
	}); err != nil {
		return fmt.Errorf("set home mode: %w", err)
	}
	return nil
}
