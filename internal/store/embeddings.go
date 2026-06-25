package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

type EmbeddingMatch struct {
	Description string
	Category    string
}

func UpsertClassificationEmbedding(ctx context.Context, db *sql.DB, workspaceID int64, description, category string, embedding []float32) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO classification_embeddings (workspace_id, description, category, embedding)
		 VALUES ($1, $2, $3, `+pgFloat8Array(embedding)+`::float8[])
		 ON CONFLICT (workspace_id, description)
		 DO UPDATE SET category = EXCLUDED.category, embedding = EXCLUDED.embedding`,
		workspaceID, description, category,
	)
	return err
}

func SearchClassificationEmbeddings(ctx context.Context, db *sql.DB, workspaceID int64, embedding []float32, limit int) ([]EmbeddingMatch, error) {
	arr := pgFloat8Array(embedding)
	rows, err := db.QueryContext(ctx,
		`SELECT description, category
		 FROM classification_embeddings
		 WHERE workspace_id = $1
		 ORDER BY cosine_similarity(embedding, `+arr+`::float8[]) DESC
		 LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EmbeddingMatch
	for rows.Next() {
		var e EmbeddingMatch
		if err := rows.Scan(&e.Description, &e.Category); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// pgFloat8Array converte um slice de float32 para a notação de array literal do PostgreSQL.
// Seguro para interpolação em SQL porque os valores são float32 da API de embeddings,
// nunca entrada do utilizador.
func pgFloat8Array(v []float32) string {
	if len(v) == 0 {
		return "ARRAY[]::float8[]"
	}
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', 8, 32)
	}
	return "'{" + strings.Join(parts, ",") + "}'"
}