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