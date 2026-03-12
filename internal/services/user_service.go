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
	return s.db.GetUserById(ctx, id)
}
