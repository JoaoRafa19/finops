package service

import (
	"context"

	"finops/internal/store"
)

type TourService interface {
	HasDoneTour(ctx context.Context, userID int64) (bool, error)
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
	return s.db.GetUserTourStatus(ctx, userID)
}

// O tour usa dados mock em memória (MockDashboard) — nada é gravado no banco,
// então concluir ou pular só precisa marcar a flag.
func (s *PGTourService) CompleteTour(ctx context.Context, userID int64) error {
	return s.db.SetTourDone(ctx, userID)
}

func (s *PGTourService) SkipTour(ctx context.Context, userID int64) error {
	return s.db.SetTourDone(ctx, userID)
}
