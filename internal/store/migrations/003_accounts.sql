-- This is a sample migration.

CREATE TABLE accounts (
  id BIGSERIAL PRIMARY KEY,
  workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  currency TEXT NOT NULL,
  opening_balance NUMERIC(19,4) NOT NULL DEFAULT 0,
  opening_date DATE,
  archived BOOLEAN NOT NULL DEFAULT FALSE
);


---- create above / drop below ----

drop table accounts;
