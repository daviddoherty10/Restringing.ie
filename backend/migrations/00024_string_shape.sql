-- +goose Up
ALTER TABLE string_products ADD COLUMN shape TEXT NOT NULL DEFAULT 'unknown'
    CHECK (shape IN ('unknown', 'round', 'shaped'));

-- +goose Down
ALTER TABLE string_products DROP COLUMN shape;
