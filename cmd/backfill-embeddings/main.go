// Command backfill-embeddings generates classification embeddings for
// transactions that were already categorized before the embeddings feature
// existed. It is safe to run multiple times: rows that already have an
// embedding for the same (workspace_id, description) are skipped.
//
// Usage: go run ./cmd/backfill-embeddings [-workspace <id>] [-dry-run]
package main

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/app"
	service "finops/internal/services"
	"finops/internal/store"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"
)

func main() {
	workspaceID := flag.Int64("workspace", 0, "only backfill this workspace (0 = all)")
	dryRun := flag.Bool("dry-run", false, "list candidates without calling the embedding API")
	flag.Parse()

	cfg := app.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	db, err := app.NewDB(ctx, cfg.DbURL)
	if err != nil {
		slog.Error("db_connect_failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	embSvc := service.NewEmbeddingService(cfg.EmbeddingBaseURL, cfg.EmbeddingAPIKey, cfg.EmbeddingModel)

	candidates, err := loadCandidates(ctx, db, *workspaceID)
	if err != nil {
		slog.Error("load_candidates_failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("candidates: %d\n", len(candidates))

	if *dryRun {
		for _, c := range candidates {
			fmt.Printf("  workspace=%d description=%q category=%q\n", c.WorkspaceID, c.Description, c.Category)
		}
		return
	}

	var done, failed int
	for i, c := range candidates {
		emb, err := embSvc.Embed(ctx, c.Description)
		if err != nil {
			slog.Warn("embed_failed", "workspace", c.WorkspaceID, "description", c.Description, "error", err)
			failed++
			continue
		}
		if err := store.UpsertClassificationEmbedding(ctx, db, c.WorkspaceID, c.Description, c.Category, emb); err != nil {
			slog.Warn("upsert_failed", "workspace", c.WorkspaceID, "description", c.Description, "error", err)
			failed++
			continue
		}
		done++
		if (i+1)%25 == 0 {
			fmt.Printf("  progress: %d/%d (failed=%d)\n", i+1, len(candidates), failed)
		}
	}
	fmt.Printf("done: %d, failed: %d\n", done, failed)
	if failed > 0 && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		os.Exit(1)
	}
}

type candidate struct {
	WorkspaceID int64
	Description string
	Category    string
}

// loadCandidates returns distinct (workspace, description, category) tuples
// from classified transactions that have no embedding yet.
func loadCandidates(ctx context.Context, db *sql.DB, workspaceID int64) ([]candidate, error) {
	q := `
		SELECT DISTINCT t.workspace_id, t.description, c.name
		FROM transactions t
		JOIN categories c ON c.id = t.category_id AND c.archived = FALSE
		LEFT JOIN classification_embeddings ce
		  ON ce.workspace_id = t.workspace_id
		  AND ce.description = t.description
		WHERE t.category_id IS NOT NULL
		  AND ce.description IS NULL
	`
	args := []any{}
	if workspaceID > 0 {
		q += " AND t.workspace_id = $1"
		args = append(args, workspaceID)
	}
	q += " ORDER BY t.workspace_id, t.description"

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.WorkspaceID, &c.Description, &c.Category); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
