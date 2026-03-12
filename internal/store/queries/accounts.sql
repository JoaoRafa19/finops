-- name: ListAccountsByWorkspace :many
SELECT
    id,
    workspace_id,
    name,
    type,
    currency,
    opening_balance,
    opening_date,
    archived
FROM accounts 
WHERE workspace_id = $1 
    AND archived = FALSE
ORDER BY name ASC;

-- name: ListAccountSummariesByWorkspace :many
SELECT
    a.id,
    a.workspace_id,
    a.name,
    a.type,
    a.currency,
    a.opening_balance,
    a.opening_date,
    a.archived,
    (
        a.opening_balance +
        COALESCE(
            SUM(
                CASE
                    WHEN t.direction = 'credit' THEN t.amount
                    ELSE -t.amount
                END
            ),
            0
        )
    )::float8 AS current_balance
FROM accounts a
LEFT JOIN transactions t
    ON t.account_id = a.id
    AND t.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
    AND a.archived = FALSE
GROUP BY
    a.id,
    a.workspace_id,
    a.name,
    a.type,
    a.currency,
    a.opening_balance,
    a.opening_date,
    a.archived
ORDER BY a.name ASC;

-- name: GetAccountByID :one 
SELECT
    id,
    workspace_id,
    name,
    type,
    currency,
    opening_balance,
    opening_date,
    archived
FROM accounts
WHERE 
    id = $1
AND archived = FALSE;

-- name: GetAccountByWorkspaceAndID :one
SELECT
    id,
    workspace_id,
    name,
    type,
    currency,
    opening_balance,
    opening_date,
    archived
FROM accounts
WHERE workspace_id = $1
    AND id = $2
    AND archived = FALSE;

-- name: CreateAccount :one 
INSERT INTO accounts
    (
    workspace_id,
    name,
    type,
    currency,
    opening_balance,
    opening_date
    )
VALUES
    ($1,$2,$3,$4,$5,$6)
RETURNING *;

-- name: UpdateAccount :one
UPDATE accounts
SET
    name = $3,
    type = $4,
    currency = $5,
    opening_balance = $6,
    opening_date = $7
WHERE workspace_id = $1
    AND id = $2
    AND archived = FALSE
RETURNING *;
