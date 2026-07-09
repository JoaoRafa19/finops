-- name: ListCommitments :many
SELECT
    c.id,
    c.workspace_id,
    c.account_id,
    c.name,
    c.kind,
    c.monthly_value,
    c.start_month,
    c.end_month,
    c.notes,
    a.name AS account_name
FROM commitments c
LEFT JOIN accounts a ON a.id = c.account_id
WHERE c.workspace_id = $1
ORDER BY c.start_month, c.name;

-- name: CreateCommitment :one
INSERT INTO commitments (
    workspace_id,
    account_id,
    name,
    kind,
    monthly_value,
    start_month,
    end_month,
    notes
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id, workspace_id, account_id, name, kind, monthly_value, start_month, end_month, notes, created_at;

-- name: GetCommitmentByWorkspaceAndID :one
SELECT id, workspace_id, account_id, name, kind, monthly_value, start_month, end_month, notes, created_at
FROM commitments
WHERE workspace_id = $1 AND id = $2;

-- name: UpdateCommitment :exec
UPDATE commitments SET
    account_id = $3,
    name = $4,
    kind = $5,
    monthly_value = $6,
    start_month = $7,
    end_month = $8,
    notes = $9
WHERE workspace_id = $1 AND id = $2;

-- name: DeleteCommitment :exec
DELETE FROM commitments WHERE workspace_id = $1 AND id = $2;

-- name: GetProjectionSettings :one
SELECT workspace_id, monthly_income, variable_expense, opening_balance, horizon_start,
    property_value, down_payment_monthly, down_payment_months, financing_annual_rate,
    financing_term_years, share_pct, updated_at
FROM projection_settings
WHERE workspace_id = $1;

-- name: UpsertProjectionSettings :one
INSERT INTO projection_settings (
    workspace_id, monthly_income, variable_expense, opening_balance, horizon_start,
    property_value, down_payment_monthly, down_payment_months, financing_annual_rate,
    financing_term_years, share_pct, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
ON CONFLICT (workspace_id) DO UPDATE SET
    monthly_income = EXCLUDED.monthly_income,
    variable_expense = EXCLUDED.variable_expense,
    opening_balance = EXCLUDED.opening_balance,
    horizon_start = EXCLUDED.horizon_start,
    property_value = EXCLUDED.property_value,
    down_payment_monthly = EXCLUDED.down_payment_monthly,
    down_payment_months = EXCLUDED.down_payment_months,
    financing_annual_rate = EXCLUDED.financing_annual_rate,
    financing_term_years = EXCLUDED.financing_term_years,
    share_pct = EXCLUDED.share_pct,
    updated_at = now()
RETURNING workspace_id, monthly_income, variable_expense, opening_balance, horizon_start,
    property_value, down_payment_monthly, down_payment_months, financing_annual_rate,
    financing_term_years, share_pct, updated_at;

-- name: ListVariableExpenseOverrides :many
SELECT workspace_id, month, value
FROM variable_expense_overrides
WHERE workspace_id = $1
ORDER BY month;

-- name: UpsertVariableExpenseOverride :exec
INSERT INTO variable_expense_overrides (workspace_id, month, value)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, month) DO UPDATE SET value = EXCLUDED.value;

-- name: DeleteVariableExpenseOverride :exec
DELETE FROM variable_expense_overrides WHERE workspace_id = $1 AND month = $2;

-- name: FindSimilarTransactions :many
SELECT id, posted_on, description, amount, direction
FROM transactions
WHERE workspace_id = $1
  AND account_id = $2
  AND amount = $3
  AND posted_on BETWEEN $4 AND $5;
