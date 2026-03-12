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
