-- +goose Up
-- Full-bed set/reel tracking must not implicitly require a separate hybrid lot.
ALTER TABLE stringer_inventory ADD COLUMN track_hybrid_stock BOOLEAN NOT NULL DEFAULT false;
-- Preserve businesses already using explicit hybrid portions and all pending
-- hybrid allocations (including older reel-backed orders). Normal set/reel-only
-- offerings use their shared manual availability for hybrid components.
UPDATE stringer_inventory i SET track_hybrid_stock=true
WHERE (i.track_stock AND EXISTS(SELECT 1 FROM inventory_lots l WHERE l.inventory_id=i.id AND l.kind='half_racket'))
 OR EXISTS(SELECT 1 FROM inventory_reservations r JOIN inventory_lots l ON l.id=r.lot_id
           WHERE l.inventory_id=i.id AND r.component IN ('mains','crosses') AND r.state='reserved');

-- +goose Down
ALTER TABLE stringer_inventory DROP COLUMN track_hybrid_stock;
