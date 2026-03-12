-- name: CreateTransaction :one
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
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ListRecentTransactionsByWorkspace :many
SELECT
    t.id,
    t.workspace_id,
    t.account_id,
    a.name AS account_name,
    t.category_id,
    t.posted_on,
    t.description,
    t.amount,
    t.direction,
    t.currency,
    t.transfer_group_id,
    t.external_fitid,
    t.source,
    t.created_at
FROM transactions t
INNER JOIN accounts a
    ON a.id = t.account_id
WHERE t.workspace_id = $1
ORDER BY t.posted_on DESC, t.id DESC
LIMIT $2;
