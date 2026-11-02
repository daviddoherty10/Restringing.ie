-- +goose Up
-- Retain strict tracking for existing offerings and their reservations.
ALTER TABLE stringer_inventory ADD COLUMN track_stock BOOLEAN NOT NULL DEFAULT true;
-- A new offering only needs a full-bed material price and manual availability.
ALTER TABLE stringer_inventory ALTER COLUMN track_stock SET DEFAULT false;
-- Earlier PR26 component-price columns remain unused for historical compatibility.
-- Full-bed prices are the sole source for derived hybrid prices; stock is not converted.

-- +goose Down
ALTER TABLE stringer_inventory DROP COLUMN track_stock;
