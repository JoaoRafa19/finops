-- Write your migrate up statements here

ALTER TABLE users ADD COLUMN has_done_tour BOOLEAN NOT NULL DEFAULT FALSE;

---- create above / drop below ----

ALTER TABLE users DROP COLUMN has_done_tour;
