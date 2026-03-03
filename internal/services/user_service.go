package service

import (
	"context"
	"finops/internal/store"
)

type UserService struct {
	db *store.Queries
}

func NewUserService(q *store.Queries) *UserService {
	return &UserService{db: q}
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (store.User, error) {
	return s.db.GetUserById(ctx, id)
}
