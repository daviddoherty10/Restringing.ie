-- +goose Up
-- A half-racket portion is a whole component serving, not a measured cut length.
-- No existing metre/set balances are converted or duplicated automatically.
ALTER TABLE racket_models DROP CONSTRAINT hybrid_lengths;
ALTER TABLE inventory_lots DROP CONSTRAINT inventory_lots_kind_check,
    DROP CONSTRAINT inventory_lots_check1,
    ADD CONSTRAINT inventory_lots_kind_check CHECK (kind IN ('set','reel','half_racket')),
    ADD CONSTRAINT inventory_lots_format_check CHECK (
        (kind IN ('reel','half_racket') AND set_length_mm=0) OR
        (kind='set' AND set_length_mm BETWEEN 1000 AND 30000));
ALTER TABLE stringer_inventory ADD COLUMN half_racket_price_cents INTEGER
    CHECK (half_racket_price_cents BETWEEN 0 AND 1000000);
-- Preserve legacy PR26 fields for databases that already applied migration 10.
-- Equal old prices can be carried over exactly; differing prices need a deliberate choice.
UPDATE stringer_inventory SET half_racket_price_cents=mains_price_cents
    WHERE mains_price_cents IS NOT NULL AND mains_price_cents=crosses_price_cents;

-- +goose Down
-- This intentionally refuses rollback while half-racket lots exist, rather than
-- deleting their stock, ledger or order allocations. Prefer a forward fix/backup.
ALTER TABLE inventory_lots DROP CONSTRAINT inventory_lots_kind_check,
    DROP CONSTRAINT inventory_lots_format_check,
    ADD CONSTRAINT inventory_lots_kind_check CHECK (kind IN ('set','reel')),
    ADD CONSTRAINT inventory_lots_check1 CHECK (
        (kind='reel' AND set_length_mm=0) OR
        (kind='set' AND set_length_mm BETWEEN 1000 AND 30000));
ALTER TABLE stringer_inventory DROP COLUMN half_racket_price_cents;
ALTER TABLE racket_models ADD CONSTRAINT hybrid_lengths CHECK (
 (mains_length_mm=0 AND crosses_length_mm=0 AND hybrid_technical_source='') OR
 (mains_length_mm BETWEEN 1000 AND 30000 AND crosses_length_mm BETWEEN 1000 AND 30000
 AND mains_length_mm+crosses_length_mm <= 30000 AND 4=ANY(allowed_knots)
 AND length(trim(hybrid_technical_source)) BETWEEN 1 AND 1000));
