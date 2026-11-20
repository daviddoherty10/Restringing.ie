-- +goose Up
ALTER TABLE stringer_inventory ADD COLUMN removed BOOLEAN NOT NULL DEFAULT false
    CHECK (NOT removed OR NOT available);

-- +goose Down
ALTER TABLE stringer_inventory DROP COLUMN removed;
