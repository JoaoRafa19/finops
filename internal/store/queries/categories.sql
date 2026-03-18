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