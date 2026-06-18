-- name: ListTransactionsFiltered :many
SELECT t.id, t.workspace_id, t.account_id, a.name AS account_name,
       t.category_id, COALESCE(c.name,'') AS category_name,
       t.posted_on, t.description, t.amount, t.direction,
       t.currency, t.transfer_group_id, t.source, t.created_at
FROM transactions t
JOIN accounts a ON a.id = t.account_id
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.workspace_id = $1
  AND t.source <> 'adjustment'
  AND t.transfer_group_id IS NULL
  AND (sqlc.narg('account_id')::bigint  IS NULL OR t.account_id  = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id')::bigint IS NULL OR t.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('direction')::text     IS NULL OR t.direction   = sqlc.narg('direction'))
  AND (sqlc.narg('from_date')::date     IS NULL OR t.posted_on  >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date       IS NULL OR t.posted_on  <= sqlc.narg('to_date'))
ORDER BY t.posted_on DESC, t.id DESC
LIMIT $2 OFFSET $3;

-- name: GetBalanceBefore :one
SELECT COALESCE(SUM(CASE WHEN direction='credit' THEN amount::numeric
                         ELSE -amount::numeric END), 0)::float8 AS balance
FROM transactions WHERE workspace_id = $1 AND posted_on < $2;


-- name: GetBalanceHistoryByMonth :many
SELECT date_trunc('month', posted_on)::date AS month,
       SUM(CASE WHEN direction='credit' THEN amount::numeric ELSE -amount::numeric END)::float8 AS net_change
FROM transactions
WHERE workspace_id = $1
  AND posted_on >= $2 AND posted_on <= $3
GROUP BY month ORDER BY month ASC;

-- name: GetMonthlyComparison :many
SELECT date_trunc('month', posted_on)::date AS month,
       SUM(CASE WHEN direction='credit' THEN amount::numeric ELSE 0 END)::float8 AS income,
       SUM(CASE WHEN direction='debit'  THEN amount::numeric ELSE 0 END)::float8 AS expenses
FROM transactions
WHERE workspace_id = $1
  AND source <> 'adjustment'
  AND transfer_group_id IS NULL
  AND posted_on >= $2 AND posted_on <= $3
GROUP BY month ORDER BY month ASC;


-- name: GetSpendingByCategoryForPeriod :many
SELECT COALESCE(c.name, 'Sem categoria') AS category_name,
       SUM(t.amount::numeric)::float8 AS total
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.workspace_id = $1
  AND t.direction = 'debit'
  AND t.source <> 'adjustment'
  AND t.transfer_group_id IS NULL
  AND t.posted_on >= $2 AND t.posted_on <= $3
GROUP BY c.name ORDER BY total DESC;

-- name: GetSpendingByCategoryByMonth :many
SELECT date_trunc('month', t.posted_on)::date AS month,
       COALESCE(c.name, 'Sem categoria') AS category_name,
       SUM(t.amount::numeric)::float8 AS total
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.workspace_id = $1
  AND t.direction = 'debit'
  AND t.source <> 'adjustment'
  AND t.transfer_group_id IS NULL
  AND t.posted_on >= $2 AND t.posted_on <= $3
GROUP BY month, c.name
ORDER BY month ASC, total DESC;

-- name: GetTopExpenses :many
SELECT t.id, a.name AS account_name,
       COALESCE(c.name, 'Sem categoria') AS category_name,
       t.posted_on, t.description,
       t.amount::float8 AS amount
FROM transactions t
JOIN accounts a ON a.id = t.account_id
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.workspace_id = $1
  AND t.direction = 'debit'
  AND t.source <> 'adjustment'
  AND t.transfer_group_id IS NULL
  AND t.posted_on >= $2 AND t.posted_on <= $3
ORDER BY t.amount::numeric DESC
LIMIT $4;