-- +goose Up
-- Existing services keep standard turnaround until their owner opts in.
ALTER TABLE stringer_services
    ADD COLUMN express BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN express_fee_cents INTEGER NOT NULL DEFAULT 0
        CHECK (express_fee_cents BETWEEN 0 AND 1000000),
    ADD COLUMN express_turnaround TEXT NOT NULL DEFAULT ''
        CHECK (length(express_turnaround) <= 160),
    ADD CONSTRAINT express_requires_turnaround CHECK
        (NOT express OR length(btrim(express_turnaround)) > 0);

-- Existing orders/quotes retain their JSON snapshots; there is no retroactive fee.
-- +goose Down
ALTER TABLE stringer_services
    DROP CONSTRAINT express_requires_turnaround,
    DROP COLUMN express_turnaround,
    DROP COLUMN express_fee_cents,
    DROP COLUMN express;
