-- +goose Up
CREATE TABLE grip_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    brand TEXT NOT NULL CHECK (length(btrim(brand)) BETWEEN 1 AND 100),
    model TEXT NOT NULL CHECK (length(btrim(model)) BETWEEN 1 AND 120),
    kind TEXT NOT NULL CHECK (kind IN ('overgrip','replacement_grip')),
    colour TEXT NOT NULL DEFAULT '' CHECK (length(colour) <= 80),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    active BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id),
    creation_key TEXT CHECK(length(creation_key) BETWEEN 16 AND 100),
    creation_request JSONB,
    UNIQUE(created_by,creation_key),
    CHECK ((created_by IS NULL AND creation_key IS NULL AND creation_request IS NULL)
        OR (created_by IS NOT NULL AND creation_key IS NOT NULL AND creation_request IS NOT NULL))
);
CREATE UNIQUE INDEX grip_products_identity_idx ON grip_products(lower(brand),lower(model),kind,lower(colour));
CREATE INDEX grip_products_catalogue_idx ON grip_products(active,kind,brand,id);
CREATE TABLE grip_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    grip_product_id UUID NOT NULL REFERENCES grip_products(id),
    fitted_price_cents INTEGER NOT NULL CHECK (fitted_price_cents BETWEEN 0 AND 1000000),
    available BOOLEAN NOT NULL DEFAULT false,
    quantity INTEGER NOT NULL DEFAULT 0 CHECK (quantity BETWEEN 0 AND 1000000),
    reserved INTEGER NOT NULL DEFAULT 0 CHECK (reserved BETWEEN 0 AND quantity),
    UNIQUE(stringer_id,grip_product_id)
);
CREATE TABLE grip_inventory_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES grip_inventory(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    quantity_delta INTEGER NOT NULL,
    reserved_delta INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 500),
    order_id UUID REFERENCES orders(id),
    idempotency_key TEXT CHECK (length(idempotency_key) BETWEEN 16 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(actor_id,idempotency_key)
);
CREATE INDEX grip_movements_inventory_idx ON grip_inventory_movements(inventory_id,created_at DESC,id);

-- One order can now reserve multiple components. Existing reservations keep
-- their identity, balances, states and ledger history as full-bed stringing.
ALTER TABLE inventory_reservations
    DROP CONSTRAINT inventory_reservations_pkey,
    ADD COLUMN component TEXT NOT NULL DEFAULT 'full_bed'
        CHECK (component IN ('full_bed','mains','crosses','grip')),
    ADD COLUMN grip_inventory_id UUID REFERENCES grip_inventory(id),
    ALTER COLUMN lot_id DROP NOT NULL,
    ADD PRIMARY KEY(order_id,component),
    ADD CONSTRAINT reservation_resource CHECK (
        (component='grip' AND lot_id IS NULL AND grip_inventory_id IS NOT NULL AND quantity=1)
        OR (component IN ('full_bed','mains','crosses') AND lot_id IS NOT NULL AND grip_inventory_id IS NULL)
    );
CREATE INDEX inventory_reservations_grip_idx ON inventory_reservations(grip_inventory_id) WHERE grip_inventory_id IS NOT NULL;

-- +goose Down
-- Rollback deliberately removes the add-on records; historical order JSON is
-- retained. Future mains/crosses allocations must be unwound before this down.
DELETE FROM inventory_reservations WHERE component='grip';
ALTER TABLE inventory_reservations
    DROP CONSTRAINT reservation_resource,
    DROP CONSTRAINT inventory_reservations_pkey,
    DROP COLUMN grip_inventory_id,
    DROP COLUMN component,
    ALTER COLUMN lot_id SET NOT NULL,
    ADD PRIMARY KEY(order_id);
DROP TABLE grip_inventory_movements, grip_inventory, grip_products;
