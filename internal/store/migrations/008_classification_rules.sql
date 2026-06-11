-- Write your migrate up statements here
CREATE TABLE classification_rules (
    id           BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    keyword      TEXT NOT NULL,
    category_id  BIGINT NOT NULL REFERENCES categories(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rule_workspace_keyword UNIQUE (workspace_id, keyword)
);

CREATE INDEX idx_rules_workspace ON classification_rules(workspace_id);

---- create above / drop below ----

DROP TABLE classification_rules;
