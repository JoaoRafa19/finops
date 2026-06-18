-- Write your migrate up statements here

CREATE OR REPLACE FUNCTION nullify_archived_category_transactions()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.archived = true AND OLD.archived = false THEN
    UPDATE transactions
    SET category_id = NULL
    WHERE workspace_id = NEW.workspace_id
      AND category_id = NEW.id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_category_archive
AFTER UPDATE OF archived ON categories
FOR EACH ROW
EXECUTE FUNCTION nullify_archived_category_transactions();

-- Limpa transacoes que ja referenciam categorias arquivadas (dados existentes)
UPDATE transactions t
SET category_id = NULL
WHERE EXISTS (
  SELECT 1 FROM categories c
  WHERE c.id = t.category_id
    AND c.archived = true
);

---- create above / drop below ----

DROP TRIGGER IF EXISTS trg_category_archive ON categories;
DROP FUNCTION IF EXISTS nullify_archived_category_transactions();
