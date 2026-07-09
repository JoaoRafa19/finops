-- Write your migrate up statements here

-- Preferências de UI por usuário. home_mode controla a densidade do dashboard:
-- 'simple' (poucos números, para quem não tem costume) vs 'advanced' (completo).
CREATE TABLE user_settings (
    user_id    BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    home_mode  TEXT NOT NULL DEFAULT 'simple' CHECK (home_mode IN ('simple','advanced')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

---- create above / drop below ----

DROP TABLE IF EXISTS user_settings;
