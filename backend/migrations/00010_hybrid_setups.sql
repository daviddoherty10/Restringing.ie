-- +goose Up
ALTER TABLE stringer_services ADD COLUMN hybrid BOOLEAN NOT NULL DEFAULT false,
    ADD CONSTRAINT hybrid_four_knots CHECK (NOT hybrid OR four_knots);
ALTER TABLE racket_models ADD COLUMN mains_length_mm INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN crosses_length_mm INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN hybrid_technical_source TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT hybrid_lengths CHECK (
        (mains_length_mm=0 AND crosses_length_mm=0 AND hybrid_technical_source='') OR
        (mains_length_mm BETWEEN 1000 AND 30000 AND crosses_length_mm BETWEEN 1000 AND 30000
         AND mains_length_mm+crosses_length_mm <= 30000 AND 4=ANY(allowed_knots)
         AND length(trim(hybrid_technical_source)) BETWEEN 1 AND 1000));
ALTER TABLE stringer_inventory ADD COLUMN mains_price_cents INTEGER CHECK (mains_price_cents BETWEEN 0 AND 1000000),
    ADD COLUMN crosses_price_cents INTEGER CHECK (crosses_price_cents BETWEEN 0 AND 1000000);
-- Prices deliberately remain NULL: a full-bed price does not define component prices.
-- Existing lot formats are unchanged. The first hybrid release allocates reels only.

-- +goose Down
ALTER TABLE stringer_inventory DROP COLUMN mains_price_cents, DROP COLUMN crosses_price_cents;
ALTER TABLE racket_models DROP CONSTRAINT hybrid_lengths, DROP COLUMN mains_length_mm,
    DROP COLUMN crosses_length_mm, DROP COLUMN hybrid_technical_source;
ALTER TABLE stringer_services DROP CONSTRAINT hybrid_four_knots, DROP COLUMN hybrid;
