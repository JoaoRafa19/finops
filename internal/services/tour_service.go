package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"finops/internal/store"
)

type TourService interface {
	HasDoneTour(ctx context.Context, userID int64) (bool, error)
	SeedMockData(ctx context.Context, userID int64) error
	CompleteTour(ctx context.Context, userID int64) error
	SkipTour(ctx context.Context, userID int64) error
}

type PGTourService struct {
	db *store.Queries
	ws WorkspaceService
}

func NewPGTourService(db *store.Queries, ws WorkspaceService) TourService {
	return &PGTourService{db: db, ws: ws}
}

func (s *PGTourService) HasDoneTour(ctx context.Context, userID int64) (bool, error) {
	done, err := s.db.GetUserTourStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	return done, nil
}

func (s *PGTourService) SeedMockData(ctx context.Context, userID int64) error {
	workspace, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("tour seed: workspace lookup: %w", err)
	}
	wsID := workspace.ID

	// Contas
	checking, err := s.db.CreateAccount(ctx, store.CreateAccountParams{
		WorkspaceID: wsID, Name: "Conta Corrente", Type: "checking", Currency: "BRL",
	})
	if err != nil {
		return fmt.Errorf("tour seed: create checking account: %w", err)
	}

	credit, err := s.db.CreateAccount(ctx, store.CreateAccountParams{
		WorkspaceID: wsID, Name: "Cartão de Crédito", Type: "credit_card", Currency: "BRL",
	})
	if err != nil {
		return fmt.Errorf("tour seed: create credit account: %w", err)
	}

	// Categorias
	cats := []store.CreateCategoryParams{
		{WorkspaceID: wsID, Name: "Alimentação", Kind: "expense"},
		{WorkspaceID: wsID, Name: "Transporte", Kind: "expense"},
		{WorkspaceID: wsID, Name: "Lazer", Kind: "expense"},
		{WorkspaceID: wsID, Name: "Assinaturas", Kind: "expense"},
		{WorkspaceID: wsID, Name: "Salário", Kind: "income"},
	}
	catIDs := make(map[string]int64)
	for _, c := range cats {
		created, err := s.db.CreateCategory(ctx, c)
		if err != nil {
			return fmt.Errorf("tour seed: create category %s: %w", c.Name, err)
		}
		catIDs[c.Name] = created.ID
	}

	// Transações (últimos 2 meses, mix de débito/crédito, algumas sem categoria)
	now := time.Now()
	catID := func(name string) sql.NullInt64 {
		return sql.NullInt64{Int64: catIDs[name], Valid: true}
	}
	noCat := sql.NullInt64{}

	txs := []struct {
		account  int64
		postedOn time.Time
		desc     string
		amount   string
		dir      string
		cat      sql.NullInt64
	}{
		{checking.ID, now.AddDate(0, -1, -20), "Salário Maio", "7500.00", "credit", catID("Salário")},
		{checking.ID, now.AddDate(0, -1, -15), "Supermercado Extra", "380.50", "debit", catID("Alimentação")},
		{checking.ID, now.AddDate(0, -1, -14), "Posto de gasolina", "200.00", "debit", catID("Transporte")},
		{credit.ID, now.AddDate(0, -1, -12), "Netflix", "55.90", "debit", catID("Assinaturas")},
		{credit.ID, now.AddDate(0, -1, -10), "iFood - Jantar", "89.90", "debit", catID("Alimentação")},
		{checking.ID, now.AddDate(0, -1, -8), "Cinema", "75.00", "debit", catID("Lazer")},
		{checking.ID, now.AddDate(0, -1, -5), "Farmácia", "124.30", "debit", noCat},
		{checking.ID, now.AddDate(0, 0, -22), "Salário Junho", "7500.00", "credit", catID("Salário")},
		{checking.ID, now.AddDate(0, 0, -18), "Supermercado Pão de Açúcar", "520.70", "debit", catID("Alimentação")},
		{credit.ID, now.AddDate(0, 0, -15), "Spotify", "21.90", "debit", catID("Assinaturas")},
		{checking.ID, now.AddDate(0, 0, -12), "Uber", "45.00", "debit", catID("Transporte")},
		{credit.ID, now.AddDate(0, 0, -10), "Amazon - Livro técnico", "89.99", "debit", noCat},
		{checking.ID, now.AddDate(0, 0, -8), "Show de rock", "180.00", "debit", catID("Lazer")},
		{credit.ID, now.AddDate(0, 0, -5), "Dentista", "350.00", "debit", noCat},
		{checking.ID, now.AddDate(0, 0, -3), "Almoço executivo", "38.50", "debit", noCat},
	}

	for _, t := range txs {
		_, err := s.db.CreateTransaction(ctx, store.CreateTransactionParams{
			WorkspaceID: wsID,
			AccountID:   t.account,
			CategoryID:  t.cat,
			PostedOn:    t.postedOn,
			Description: t.desc,
			Amount:      t.amount,
			Direction:   t.dir,
			Currency:    "BRL",
			Source:      "manual",
		})
		if err != nil {
			return fmt.Errorf("tour seed: create transaction %q: %w", t.desc, err)
		}
	}

	// Ajuste de saldo das contas via transação de ajuste
	s.seedAdjustment(ctx, wsID, checking.ID, "5000.00", "credit")
	s.seedAdjustment(ctx, wsID, credit.ID, "800.00", "debit")

	return nil
}

func (s *PGTourService) seedAdjustment(ctx context.Context, wsID, accountID int64, amount, dir string) {
	_, _ = s.db.CreateTransaction(ctx, store.CreateTransactionParams{
		WorkspaceID: wsID,
		AccountID:   accountID,
		PostedOn:    time.Now().AddDate(0, -2, 0),
		Description: "Saldo inicial",
		Amount:      amount,
		Direction:   dir,
		Currency:    "BRL",
		Source:      "adjustment",
	})
}

func (s *PGTourService) CompleteTour(ctx context.Context, userID int64) error {
	if err := s.cleanWorkspace(ctx, userID); err != nil {
		return err
	}
	return s.db.SetTourDone(ctx, userID)
}

func (s *PGTourService) SkipTour(ctx context.Context, userID int64) error {
	return s.db.SetTourDone(ctx, userID)
}

func (s *PGTourService) cleanWorkspace(ctx context.Context, userID int64) error {
	workspace, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return err
	}
	wsID := workspace.ID
	if err := s.db.DeleteWorkspaceData(ctx, wsID); err != nil {
		return err
	}
	if err := s.db.DeleteWorkspaceAccounts(ctx, wsID); err != nil {
		return err
	}
	return s.db.DeleteWorkspaceCategories(ctx, wsID)
}
