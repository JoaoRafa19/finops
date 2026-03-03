-- Write your migrate up statements here
CREATE TABLE workspaces (
  id BIGSERIAL PRIMARY KEY,
  owner_user_id BIGINT NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  default_currency TEXT NOT NULL DEFAULT 'BRL',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
---- create above / drop below ----
drop table workspaces;
-- Write your migrate down statements here. If this migration is irreversible
-- Then delete the separator line above.
