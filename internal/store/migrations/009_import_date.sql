-- Write your migrate up statements here
CREATE TABLE imports (
    id           BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    imported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rule_workspace_keyword UNIQUE (workspace_id, id)
);

CREATE INDEX idx_imports_workspace ON imports(workspace_id);

---- create above / drop below ----

DROP TABLE imports;
