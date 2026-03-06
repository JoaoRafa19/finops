-- name: GetWorkSpaceByOwnerUserID :one 
SELECT
    id,
    owner_user_id,
    name,
    default_currency,
    created_at
FROM 
    workspaces 
WHERE owner_user_id = $1
LIMIT 1;

-- name: CreateWorkspace :one 
INSERT INTO workspaces (
    owner_user_id,
    name,
    default_currency
) VALUES (
    $1, $2, $3
)
RETURNING id, owner_user_id, name, default_currency, created_at;