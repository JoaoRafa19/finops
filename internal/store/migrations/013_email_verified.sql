-- Write your migrate up statements here
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ NULL;

-- Marca todos os usuários existentes como já verificados: eles já usam o sistema
-- e não deve haver quebra retroativa. Novos cadastros terão NULL até verificar.
UPDATE users SET email_verified_at = NOW() WHERE email_verified_at IS NULL;

---- create above / drop below ----

ALTER TABLE users DROP COLUMN email_verified_at;
