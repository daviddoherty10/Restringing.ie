-- +goose Up
CREATE TABLE string_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    brand TEXT NOT NULL CHECK (length(brand) BETWEEN 1 AND 100),
    model TEXT NOT NULL CHECK (length(model) BETWEEN 1 AND 120),
    material TEXT NOT NULL CHECK (material IN ('multifilament','synthetic_gut','polyester','natural_gut','nylon','hybrid')),
    sport TEXT NOT NULL CHECK (sport IN ('tennis','badminton','squash')),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    comfort SMALLINT NOT NULL CHECK (comfort BETWEEN 1 AND 5),
    control SMALLINT NOT NULL CHECK (control BETWEEN 1 AND 5),
    power SMALLINT NOT NULL CHECK (power BETWEEN 1 AND 5),
    spin SMALLINT NOT NULL CHECK (spin BETWEEN 1 AND 5),
    durability SMALLINT NOT NULL CHECK (durability BETWEEN 1 AND 5),
    max_prestretch_percent SMALLINT NOT NULL DEFAULT 0 CHECK (max_prestretch_percent BETWEEN 0 AND 30),
    active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(brand,model,sport)
);
CREATE INDEX string_products_filter_idx ON string_products(sport,material,brand,id);
CREATE TABLE string_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES string_products(id),
    gauge_mm NUMERIC(4,2) NOT NULL CHECK (gauge_mm BETWEEN 0.40 AND 2.00),
    colour TEXT NOT NULL DEFAULT '' CHECK (length(colour) <= 60),
    active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(product_id,gauge_mm,colour)
);

-- Manufacturer/model-specific constraints must be curated before a live quote.
-- No universal tennis tension range is applied to badminton or squash rackets.
CREATE TABLE racket_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    brand TEXT NOT NULL CHECK (length(brand) BETWEEN 1 AND 100),
    model TEXT NOT NULL CHECK (length(model) BETWEEN 1 AND 120),
    sport TEXT NOT NULL CHECK (sport IN ('tennis','badminton','squash')),
    min_tension_centi_kg INTEGER NOT NULL CHECK (min_tension_centi_kg > 0),
    max_tension_centi_kg INTEGER NOT NULL,
    step_centi_kg INTEGER NOT NULL CHECK (step_centi_kg BETWEEN 1 AND 100),
    max_difference_centi_kg INTEGER NOT NULL CHECK (max_difference_centi_kg >= 0),
    allowed_knots INTEGER[] NOT NULL CHECK (cardinality(allowed_knots) > 0 AND allowed_knots <@ ARRAY[2,4]),
    different_tensions_require_four_knots BOOLEAN NOT NULL DEFAULT true,
    string_length_mm INTEGER NOT NULL CHECK (string_length_mm BETWEEN 1000 AND 30000),
    technical_source TEXT NOT NULL CHECK (length(technical_source) BETWEEN 1 AND 1000),
    verified BOOLEAN NOT NULL DEFAULT false,
    CHECK (max_tension_centi_kg >= min_tension_centi_kg AND max_tension_centi_kg <= 10000),
    UNIQUE(brand,model,sport)
);
CREATE TABLE stringer_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    sport TEXT NOT NULL CHECK (sport IN ('tennis','badminton','squash')),
    active BOOLEAN NOT NULL DEFAULT true,
    labour_cents INTEGER NOT NULL CHECK (labour_cents BETWEEN 0 AND 1000000),
    own_strings BOOLEAN NOT NULL DEFAULT false,
    own_strings_cents INTEGER NOT NULL DEFAULT 0 CHECK (own_strings_cents BETWEEN 0 AND 1000000),
    stencil BOOLEAN NOT NULL DEFAULT false,
    stencil_cents INTEGER NOT NULL DEFAULT 0 CHECK (stencil_cents BETWEEN 0 AND 1000000),
    two_knots BOOLEAN NOT NULL DEFAULT false,
    two_knots_cents INTEGER NOT NULL DEFAULT 0 CHECK (two_knots_cents BETWEEN 0 AND 1000000),
    four_knots BOOLEAN NOT NULL DEFAULT true,
    four_knots_cents INTEGER NOT NULL DEFAULT 0 CHECK (four_knots_cents BETWEEN 0 AND 1000000),
    different_tensions BOOLEAN NOT NULL DEFAULT false,
    different_tensions_cents INTEGER NOT NULL DEFAULT 0 CHECK (different_tensions_cents BETWEEN 0 AND 1000000),
    min_tension_centi_kg INTEGER NOT NULL CHECK (min_tension_centi_kg > 0),
    max_tension_centi_kg INTEGER NOT NULL CHECK (max_tension_centi_kg <= 10000),
    step_centi_kg INTEGER NOT NULL CHECK (step_centi_kg BETWEEN 1 AND 100),
    max_difference_centi_kg INTEGER NOT NULL CHECK (max_difference_centi_kg >= 0),
    UNIQUE(stringer_id,sport),
    CHECK (max_tension_centi_kg >= min_tension_centi_kg),
    CHECK (two_knots OR four_knots)
);
CREATE TABLE service_prestretch_options (
    service_id UUID NOT NULL REFERENCES stringer_services(id),
    percent INTEGER NOT NULL CHECK (percent BETWEEN 1 AND 30),
    price_cents INTEGER NOT NULL CHECK (price_cents BETWEEN 0 AND 1000000),
    PRIMARY KEY(service_id,percent)
);
CREATE TABLE stringer_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    variant_id UUID NOT NULL REFERENCES string_variants(id),
    material_price_cents INTEGER NOT NULL CHECK (material_price_cents BETWEEN 0 AND 1000000),
    available BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(stringer_id,variant_id)
);
CREATE INDEX stringer_inventory_variant_idx ON stringer_inventory(variant_id);
-- Quantities: whole sets for 'set'; integer millimetres for 'reel'.
CREATE TABLE inventory_lots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES stringer_inventory(id),
    kind TEXT NOT NULL CHECK (kind IN ('set','reel')),
    set_length_mm INTEGER NOT NULL DEFAULT 0,
    quantity INTEGER NOT NULL CHECK (quantity BETWEEN 0 AND 100000000),
    reserved INTEGER NOT NULL DEFAULT 0 CHECK (reserved >= 0 AND reserved <= quantity),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((kind='reel' AND set_length_mm=0) OR (kind='set' AND set_length_mm BETWEEN 1000 AND 30000))
);
CREATE INDEX inventory_lots_inventory_idx ON inventory_lots(inventory_id,id);
CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lot_id UUID NOT NULL REFERENCES inventory_lots(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    quantity_delta INTEGER NOT NULL,
    reserved_delta INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX inventory_movements_lot_idx ON inventory_movements(lot_id,created_at);

-- +goose Down
DROP TABLE inventory_movements, inventory_lots, stringer_inventory,
    service_prestretch_options, stringer_services, racket_models, string_variants, string_products;
