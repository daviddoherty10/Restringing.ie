-- +goose Up
ALTER TABLE string_products DROP CONSTRAINT string_products_material_check;
ALTER TABLE string_products ALTER COLUMN material DROP NOT NULL;
UPDATE string_products SET material='synthetic_gut' WHERE material='nylon';
-- A packaged pair has no single construction. Retain its identity for existing inventory and orders.
UPDATE string_products SET active=false, material=NULL WHERE material='hybrid';
ALTER TABLE string_products ADD CONSTRAINT string_products_material_check CHECK (
    (material IS NULL AND NOT active) OR
    (material IS NOT NULL AND material IN ('multifilament','synthetic_gut','polyester','natural_gut'))
);

-- Exact catalogue identities only. Preserve shapes already curated by a stringer.
UPDATE string_products AS p SET shape=models.shape
FROM (VALUES
    ('solinco','hyper g','shaped'), ('solinco','hyper g soft','shaped'),
    ('solinco','tour bite','shaped'), ('solinco','hyper g round','round'),
    ('luxilon','alu power','round'), ('msv','focus hex','shaped'), ('msv','co focus','round')
) AS models(brand,model,shape)
WHERE p.sport='tennis' AND p.material='polyester' AND p.shape='unknown'
  AND lower(trim(p.brand))=models.brand
  AND lower(trim(replace(p.model,'-',' ')))=models.model;

-- +goose Down
ALTER TABLE string_products DROP CONSTRAINT string_products_material_check;
UPDATE string_products SET material='hybrid' WHERE material IS NULL;
ALTER TABLE string_products ALTER COLUMN material SET NOT NULL;
ALTER TABLE string_products ADD CONSTRAINT string_products_material_check
    CHECK (material IN ('multifilament','synthetic_gut','polyester','natural_gut','nylon','hybrid'));
