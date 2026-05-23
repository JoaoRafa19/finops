-- Write your migrate up statements here
CREATE TABLE categories (
  id BIGSERIAL PRIMARY KEY,
  workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
  parent_id BIGINT REFERENCES categories(id),
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('expense','income','transfer')),
  archived BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE (workspace_id, name)
);
---- create above / drop below ----

drop table categories;
