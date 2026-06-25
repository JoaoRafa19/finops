-- Armazena embeddings de transações classificadas para busca semântica por similaridade de cosseno.
CREATE TABLE IF NOT EXISTS classification_embeddings (
    id           BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT  NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    description  TEXT    NOT NULL,
    category     TEXT    NOT NULL,
    embedding    FLOAT8[] NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, description)
);

CREATE INDEX IF NOT EXISTS idx_class_emb_workspace ON classification_embeddings(workspace_id);

-- Similaridade de cosseno entre dois vetores float8[].
-- Retorna 0 quando qualquer norma é zero (vetores nulos).
CREATE OR REPLACE FUNCTION cosine_similarity(a FLOAT8[], b FLOAT8[]) RETURNS FLOAT8
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE WHEN norm_a = 0 OR norm_b = 0 THEN 0
                ELSE dot / (norm_a * norm_b)
           END
    FROM (
        SELECT
            SUM(a[i] * b[i])     AS dot,
            SQRT(SUM(a[i] ^ 2))  AS norm_a,
            SQRT(SUM(b[i] ^ 2))  AS norm_b
        FROM generate_subscripts(a, 1) AS i(i)
    ) t
$$;
