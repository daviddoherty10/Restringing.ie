-- +goose Up
ALTER TABLE string_variants ADD COLUMN photo BYTEA CHECK (octet_length(photo) BETWEEN 1 AND 1048576);

-- +goose Down
ALTER TABLE string_variants DROP COLUMN photo;
