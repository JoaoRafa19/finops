-- Write your migrate up statements here
CREATE TABLE transactions (
  id BIGSERIAL PRIMARY KEY,
  workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
  account_id BIGINT NOT NULL REFERENCES accounts(id),
  category_id BIGINT REFERENCES categories(id),
  posted_on DATE NOT NULL,
  description TEXT NOT NULL,
  amount NUMERIC(19,4) NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('debit','credit')),
  currency TEXT NOT NULL,
  transfer_group_id BIGINT,
  external_fitid TEXT,
  source TEXT NOT NULL CHECK (source IN ('manual','import','adjustment')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tx_account_posted ON transactions(account_id, posted_on DESC);
CREATE UNIQUE INDEX uq_tx_fitid ON transactions(workspace_id, account_id, external_fitid)
  WHERE external_fitid IS NOT NULL;

---- create above / drop below ----

drop table transactions;         