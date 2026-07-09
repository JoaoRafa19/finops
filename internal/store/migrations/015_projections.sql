-- Write your migrate up statements here

-- Compromissos: parcelamentos, assinaturas, gastos fixos, recebíveis e eventos
-- únicos. RN-01/02/03 — todo compromisso é (valor mensal, vigência); one_off e
-- income pontual usam start=end. Fonte do total que a Projeção consome.
CREATE TABLE commitments (
    id            BIGSERIAL PRIMARY KEY,
    workspace_id  BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    account_id    BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    name          TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('installment','subscription','fixed','income','one_off')),
    monthly_value NUMERIC(14,2) NOT NULL,
    start_month   DATE NOT NULL,
    end_month     DATE,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_month IS NULL OR end_month >= start_month)
);

CREATE INDEX idx_commitments_workspace ON commitments(workspace_id);

-- Premissas globais (1 linha por workspace): renda, saldo base, gasto variável
-- padrão, janela do horizonte e parâmetros do simulador de financiamento.
CREATE TABLE projection_settings (
    workspace_id          BIGINT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    monthly_income        NUMERIC(14,2) NOT NULL DEFAULT 0,
    variable_expense      NUMERIC(14,2) NOT NULL DEFAULT 0,
    opening_balance       NUMERIC(14,2) NOT NULL DEFAULT 0,
    horizon_start         DATE,
    -- Simulador (compra de imóvel na planta)
    property_value        NUMERIC(14,2) NOT NULL DEFAULT 0,
    down_payment_monthly  NUMERIC(14,2) NOT NULL DEFAULT 0,
    down_payment_months   INTEGER NOT NULL DEFAULT 0,
    financing_annual_rate NUMERIC(6,4) NOT NULL DEFAULT 0,
    financing_term_years  INTEGER NOT NULL DEFAULT 0,
    share_pct             NUMERIC(5,4) NOT NULL DEFAULT 1,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Override do gasto variável mês a mês (item 11 da Projeção): sem override, usa
-- o padrão de projection_settings.variable_expense.
CREATE TABLE variable_expense_overrides (
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    month        DATE NOT NULL,
    value        NUMERIC(14,2) NOT NULL,
    PRIMARY KEY (workspace_id, month)
);

---- create above / drop below ----

DROP TABLE IF EXISTS variable_expense_overrides;
DROP TABLE IF EXISTS projection_settings;
DROP TABLE IF EXISTS commitments;
