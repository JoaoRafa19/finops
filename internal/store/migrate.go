package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrateUp runs all pending migrations. Compatible with the tern
// `schema_version` table (single row containing the current version), so
// `make migrate` (dev) and this auto-runner (prod) share state cleanly.
func MigrateUp(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS public.schema_version (version int4 NOT NULL)`,
	); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO public.schema_version (version)
		 SELECT 0 WHERE NOT EXISTS (SELECT 1 FROM public.schema_version)`,
	); err != nil {
		return fmt.Errorf("seed schema_version: %w", err)
	}

	var current int
	if err := db.QueryRowContext(ctx,
		`SELECT version FROM public.schema_version LIMIT 1`,
	).Scan(&current); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	type migration struct {
		version int
		name    string
	}
	var pending []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// Nome esperado: NNN_texto.sql (tern default). Extrai NNN antes do "_".
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if v > current {
			pending = append(pending, migration{version: v, name: e.Name()})
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].version < pending[j].version })

	for _, m := range pending {
		raw, err := migrationsFS.ReadFile("migrations/" + m.name)
		if err != nil {
			return fmt.Errorf("read %s: %w", m.name, err)
		}
		// Separador do tern: só a metade "up" antes do delimitador.
		up, _, _ := strings.Cut(string(raw), "---- create above / drop below ----")

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, up); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE public.schema_version SET version = $1`, m.version,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", m.name, err)
		}
	}
	return nil
}
