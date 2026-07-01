-- name: CreateCategory :one 
INSERT INTO categories (
    workspace_id,
    parent_id,
    name, 
    kind
) VALUES ($1,$2,$3,$4) RETURNING 
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    archived
;


-- name: GetCategories :many 
SELECT 
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    archived
FROM categories WHERE archived = false AND workspace_id=$1 ORDER BY name ASC;

-- name: GetCategoryByWorkspaceAndName :one
SELECT
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    archived
FROM categories
WHERE workspace_id = $1
    AND name = $2;

-- name: GetCategoryByWorkspaceAndID :one
SELECT
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    archived
FROM categories
WHERE archived = false
    AND workspace_id = $1
    AND id = $2;


-- name: DeleteCategory :exec
UPDATE categories SET archived = true WHERE workspace_id = $1 AND id = $2;

-- name: UpdateCategoryName :exec
UPDATE categories SET name = $3 WHERE workspace_id = $1 AND id = $2 AND archived = false;

-- name: UnarchiveCategory :exec
UPDATE categories SET archived = false WHERE workspace_id = $1 AND name = $2;
