-- name: GetUserById :one
SELECT id, email, password_hash, password_algo, is_admin, has_done_tour, created_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, password_algo, is_admin, has_done_tour, created_at
FROM users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, password_algo, is_admin)
VALUES ($1, $2, $3, FALSE)
RETURNING id;

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = $2, password_algo = $3 WHERE email = $1;

-- name: SetTourDone :exec
UPDATE users SET has_done_tour = TRUE WHERE id = $1;

-- name: GetUserTourStatus :one
SELECT has_done_tour FROM users WHERE id = $1;

-- name: DeleteWorkspaceData :exec
DELETE FROM transactions WHERE workspace_id = $1;

-- name: DeleteWorkspaceAccounts :exec
DELETE FROM accounts WHERE workspace_id = $1;

-- name: DeleteWorkspaceCategories :exec
DELETE FROM categories WHERE workspace_id = $1;     

