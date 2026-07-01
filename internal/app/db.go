package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(ctx context.Context, databaseUrl string) (*sql.DB, error) {
	if databaseUrl == "" {
		return nil, errors.New("DATABASE_URL is required, invalid database url")
	}

	// Log defensivo: se o DSN não parece uma URL Postgres válida, avisa cedo.
	slog.Info("db_connect_attempt", "dsn", maskDSN(databaseUrl))

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

// maskDSN devolve o DSN com a senha redigida — seguro pra log.
func maskDSN(dsn string) string {
	if dsn == "" {
		return "<empty>"
	}
	if scheme, rest, ok := strings.Cut(dsn, "://"); ok {
		if userPart, hostPart, ok := strings.Cut(rest, "@"); ok {
			if user, _, ok := strings.Cut(userPart, ":"); ok {
				return scheme + "://" + user + ":***@" + hostPart
			}
		}
	}
	// keyword=value: só devolve os "campos" sem valor
	if strings.Contains(dsn, "=") {
		var parts []string
		for kv := range strings.FieldsSeq(dsn) {
			if k, _, ok := strings.Cut(kv, "="); ok {
				parts = append(parts, k+"=***")
			}
		}
		return strings.Join(parts, " ")
	}
	return "<opaque>"
}
