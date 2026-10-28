package marketplace

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// The caller holds the business lock and owns the transaction. Multiple item
// allocations either commit together with the order, or are all rolled back.
func reserveOrderItems(ctx context.Context, tx pgx.Tx, a Actor, o Order, r Racket, v *Variant) error {
	if o.Configuration.StringSource == "stock" {
		lot, n, e := chooseLot(ctx, tx, v.InventoryID, r.StringLengthMM, true)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE inventory_lots SET reserved=reserved+$2 WHERE id=$1`, lot, n); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO inventory_reservations(order_id,component,lot_id,quantity) VALUES($1,'full_bed',$2,$3)`, o.ID, lot, n); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO inventory_movements(lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id) VALUES($1,$2,0,$3,'order reservation',$4)`, lot, a.ID, n, o.ID); e != nil {
			return e
		}
	}
	if product := o.Configuration.GripProductID; product != "" {
		var inventory string
		var enough bool
		e := tx.QueryRow(ctx, `SELECT id::text,(available AND quantity-reserved>=1) FROM grip_inventory WHERE stringer_id=$1 AND grip_product_id=$2 FOR UPDATE`, o.StringerID, product).Scan(&inventory, &enough)
		if e != nil {
			return missing(e)
		}
		if !enough {
			return problem(409, "grip_out_of_stock", "This grip has no unreserved units. Choose another grip or continue without one.")
		}
		if _, e = tx.Exec(ctx, `UPDATE grip_inventory SET reserved=reserved+1 WHERE id=$1`, inventory); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO inventory_reservations(order_id,component,grip_inventory_id,quantity) VALUES($1,'grip',$2,1)`, o.ID, inventory); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO grip_inventory_movements(inventory_id,actor_id,quantity_delta,reserved_delta,reason,order_id) VALUES($1,$2,0,1,'order reservation',$3)`, inventory, a.ID, o.ID); e != nil {
			return e
		}
	}
	return nil
}

// Transition every reservation still in the reserved state. This supports
// multiple string components without assuming that an order has one stock row.
// Hybrids will supply their own sourced component lengths and pricing later.
func transitionReservations(ctx context.Context, tx pgx.Tx, a Actor, id, status string) error {
	type allocation struct {
		component string
		lot, grip *string
		quantity  int
	}
	rows, e := tx.Query(ctx, `SELECT component,lot_id::text,grip_inventory_id::text,quantity FROM inventory_reservations WHERE order_id=$1 AND state='reserved' ORDER BY component FOR UPDATE`, id)
	if e != nil {
		return e
	}
	var items []allocation
	for rows.Next() {
		var item allocation
		if e = rows.Scan(&item.component, &item.lot, &item.grip, &item.quantity); e != nil {
			rows.Close()
			return e
		}
		items = append(items, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, item := range items {
		delta := 0
		state := "released"
		reason := "order cancellation"
		if status == "in_progress" {
			delta = -item.quantity
			state = "consumed"
			reason = "string used for order"
			if item.grip != nil {
				reason = "grip issued for order"
			}
		}
		if item.lot != nil {
			if _, e = tx.Exec(ctx, `UPDATE inventory_lots SET quantity=quantity+$2,reserved=reserved-$3 WHERE id=$1`, *item.lot, delta, item.quantity); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO inventory_movements(lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id) VALUES($1,$2,$3,$4,$5,$6)`, *item.lot, a.ID, delta, -item.quantity, reason, id); e != nil {
				return e
			}
		} else {
			if _, e = tx.Exec(ctx, `UPDATE grip_inventory SET quantity=quantity+$2,reserved=reserved-$3 WHERE id=$1`, *item.grip, delta, item.quantity); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO grip_inventory_movements(inventory_id,actor_id,quantity_delta,reserved_delta,reason,order_id) VALUES($1,$2,$3,$4,$5,$6)`, *item.grip, a.ID, delta, -item.quantity, reason, id); e != nil {
				return e
			}
		}
		if _, e = tx.Exec(ctx, `UPDATE inventory_reservations SET state=$3 WHERE order_id=$1 AND component=$2 AND state='reserved'`, id, item.component, state); e != nil {
			return e
		}
	}
	return nil
}
