-- Write your migrate up statements here
alter table accounts 
    alter column opening_balance type double precision using opening_balance::double precision;
---- create above / drop below ----

alter table accounts 
    alter column opening_balance type numeric(19,4) using opening_balance::numeric(19,4);

-- Write your migrate down statements here. If this migration is irreversible
-- Then delete the separator line above.
