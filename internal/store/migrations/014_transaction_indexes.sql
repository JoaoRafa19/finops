-- Write your migrate up statements here

-- Índice principal para queries por período no workspace (dashboard, relatórios,
-- ListTransactionsFiltered sem filtro de conta). Ordena DESC pra ORDER BY posted_on DESC.
CREATE INDEX IF NOT EXISTS idx_tx_workspace_posted ON transactions(workspace_id, posted_on DESC);

-- Índice pra filtro por categoria (usado no relatório de gastos e classificações).
CREATE INDEX IF NOT EXISTS idx_tx_workspace_category ON transactions(workspace_id, category_id)
  WHERE category_id IS NOT NULL;

-- Índice pra transações não-classificadas (query de pendentes).
CREATE INDEX IF NOT EXISTS idx_tx_workspace_uncat ON transactions(workspace_id)
  WHERE category_id IS NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_tx_workspace_uncat;
DROP INDEX IF EXISTS idx_tx_workspace_category;
DROP INDEX IF EXISTS idx_tx_workspace_posted;
