package service

import (
	"context"
	"finops/internal/store"
)

type UserService interface {
	GetUserByID(ctx context.Context, id int64) (store.User, error)
}

type PGUserService struct {
	db *store.Queries
}

func NewUserService(q *store.Queries) UserService {
	return &PGUserService{db: q}
}

func (s *PGUserService) GetUserByID(ctx context.Context, id int64) (store.User, error) {
	row, err := s.db.GetUserById(ctx, id)
	if err != nil {
		return store.User{}, err
	}
	return store.User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		PasswordAlgo: row.PasswordAlgo,
		IsAdmin:      row.IsAdmin,
		HasDoneTour:  row.HasDoneTour,
		CreatedAt:    row.CreatedAt,
	}, nil
}
