-- name: ListUnclassifiedTransactions :many
SELECT
    t.id,
    t.workspace_id,
    t.account_id,
    a.name AS account_name,
    t.posted_on,
    t.description,
    t.amount,
    t.direction,
    t.currency,
    t.source,
    t.created_at
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.workspace_id = $1
  AND t.category_id IS NULL
  AND t.source <> 'adjustment'
  AND t.transfer_group_id IS NULL
ORDER BY t.posted_on DESC, t.id DESC
LIMIT $2;

-- name: CountUnclassifiedTransactions :one
SELECT COUNT(*)::int AS total
FROM transactions
WHERE workspace_id = $1
  AND category_id IS NULL
  AND source <> 'adjustment'
  AND transfer_group_id IS NULL;

-- name: ClassifyTransaction :exec
UPDATE transactions
SET category_id = $2
WHERE id = $1
  AND workspace_id = $3;

-- name: ClassifyTransactionsByKeyword :execrows
UPDATE transactions
SET category_id = $3
WHERE workspace_id = $1
  AND category_id IS NULL
  AND transfer_group_id IS NULL
  AND source <> 'adjustment'
  AND lower(description) LIKE lower($2);

-- name: UpsertClassificationRule :exec
INSERT INTO classification_rules (workspace_id, keyword, category_id)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, keyword)
DO UPDATE SET category_id = EXCLUDED.category_id;

-- name: ListClassificationRules :many
SELECT id, workspace_id, keyword, category_id, created_at
FROM classification_rules
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: SearchCategories :many
SELECT id, workspace_id, name, kind, archived
FROM categories
WHERE workspace_id = $1
  AND archived = FALSE
  AND lower(name) LIKE lower($2)
ORDER BY name ASC
LIMIT 10;


-- name: GetTransactionDescriptionCategory :many
SELECT DISTINCT ON (lower(t.description)) t.id, t.description, c.name AS category FROM transactions t 
INNER JOIN categories c ON c.id = t.category_id
WHERE c.workspace_id = $1 AND c.archived = FALSE AND c.kind <> 'transfer' 
ORDER BY lower(t.description), t.created_at DESC  LIMIT 300 ;


-- name: InsertImportDate :exec
INSERT INTO imports (workspace_id) VALUES ($1);

-- name: LastImportDate :one 
SELECT imported_at FROM imports WHERE workspace_id = $1 ORDER BY imported_at DESC LIMIT 1;