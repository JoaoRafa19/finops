INSERT INTO transactions (
  workspace_id,
  account_id,
  category_id,
  posted_on,
  description,
  amount,
  direction,
  currency,
  transfer_group_id,
  external_fitid,
  source
)
SELECT
  workspace_id,
  id,
  NULL,
  COALESCE(opening_date, CURRENT_DATE),
  'Ajuste de saldo inicial',
  ABS(opening_balance),
  CASE
    WHEN opening_balance >= 0 THEN 'credit'
    ELSE 'debit'
  END,
  currency,
  NULL,
  NULL,
  'adjustment'
FROM accounts
WHERE opening_balance <> 0;

ALTER TABLE accounts
  DROP COLUMN opening_balance,
  DROP COLUMN opening_date;

---- create above / drop below ----

ALTER TABLE accounts
  ADD COLUMN opening_balance double precision NOT NULL DEFAULT 0,
  ADD COLUMN opening_date date;
