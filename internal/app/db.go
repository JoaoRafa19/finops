package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDbPool(ctx context.Context, databaseUrl string) (*pgxpool.Pool, error) {
	if databaseUrl == "" {
		return nil, errors.New("DATABASE_URL is required, invalid database url")
	}

	pool, err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error creating connection: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error connecting database: %w", err)
	}

	return pool, nil
}
