package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(ctx context.Context, databaseUrl string) (*sql.DB, error) {
	if databaseUrl == "" {
		return nil, errors.New("DATABASE_URL is required, invalid database url")
	}

	db, err := sql.Open("pgx", databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error opening database connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("error connecting database: %w", err)
	}

	return db, nil
}
