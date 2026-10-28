import type { Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
type S = components['schemas']
export const uid = (n: number) => `90000000-0000-4000-8000-${String(n).padStart(12, '0')}`
export const shopID = uid(101), otherShopID = uid(102)
const profile = (id: string, name: string, staff: boolean): S['ManagementProfile'] => ({ id, name, description: 'Fictional preview business', area: 'Dublin', county: 'Dublin', approval_status: 'approved', accepting_orders: true, booking_available: true, membership_role: staff ? 'staff' : 'owner', permissions: { edit_profile: !staff, edit_services: !staff, manage_inventory: true, manage_orders: true, manage_billing: !staff } })
export async function dashboardAPI(page: Page, role: 'owner' | 'staff' | 'admin' = 'owner') {
  const businesses = [profile(shopID, 'Preview Baseline', role === 'staff'), profile(otherShopID, 'Preview Racket Room', false)]
  const variants: S['Variant'][] = Array.from({ length: 24 }, (_, i) => ({ id: uid(2001 + i), product_id: uid(1001 + i), inventory_id: uid(3001 + i), brand: 'Preview', model: `String ${String(i + 1).padStart(2, '0')}`, description: '', sport: 'tennis', material: 'multifilament', gauge_mm: 1.3, colour: 'Natural', comfort: 4, control: 3, power: 3, spin: 3, durability: 3, max_prestretch_percent: 15, material_price_cents: 1099 + i, available: true }))
  const otherVariant = { ...variants[0], id: uid(2999), model: 'Other business string', material_price_cents: 2099 }
  const service: S['Service'] = { express: false, express_fee_cents: 0, express_turnaround: '', id: uid(4001), sport: 'tennis', active: true, labour_cents: 1500, own_strings: true, own_strings_cents: 0, stencil: true, stencil_cents: 300, two_knots: true, two_knots_cents: 0, four_knots: true, four_knots_cents: 100, different_tensions: true, different_tensions_cents: 100, min_tension_centi_kg: 1800, max_tension_centi_kg: 3000, step_centi_kg: 50, max_difference_centi_kg: 200, prestretch_options: { '5': 300 } }
  const services = [service]
  const order: S['Order'] = { id: uid(5001), customer_id: uid(1), stringer_id: shopID, quote_id: uid(6001), status: 'placed', payment_status: 'unpaid', payment_method: 'cash', currency: 'EUR', total_cents: 2599, configuration: { racket_model_id: uid(7001), racket_snapshot: 'Preview Tennis Model', string_source: 'stock', variant_id: variants[0].id, string_snapshot: 'Preview String 01', mains_tension_centi_kg: 2450, crosses_tension_centi_kg: 2400, knots: 4, stencil: false, prestretch_percent: 0, instructions: 'Please keep the old grip.' }, breakdown: [{ code: 'labour', label: 'Labour', amount_cents: 1500 }, { code: 'strings', label: 'Strings', amount_cents: 1099 }], created_at: new Date().toISOString(), updated_at: new Date().toISOString(), events: [{ from_status: null, to_status: 'placed', created_at: new Date().toISOString() }] }
  const lots: S['StockLot'][] = [{ id: uid(8001), inventory_id: variants[0].inventory_id!, variant_id: variants[0].id, brand: variants[0].brand, model: variants[0].model, gauge_mm: 1.3, colour: 'Natural', sport: 'tennis', kind: 'set', set_length_mm: 12000, quantity: 8, reserved: 2 }, { id: uid(8002), inventory_id: variants[1].inventory_id!, variant_id: variants[1].id, brand: variants[1].brand, model: variants[1].model, gauge_mm: 1.3, colour: 'Natural', sport: 'tennis', kind: 'reel', set_length_mm: 0, quantity: 200000, reserved: 12000 }]
  let pending = true
  const writes: { method: string; path: string; body: Record<string, unknown> }[] = []
  await page.route('**/api/v1/**', async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname.replace('/api/v1', ''), method = request.method()
    const json = (body: unknown, status = 200) => route.fulfill({ status, json: body })
    const problem = (status: number, code: string, message: string) => json({ error: { code, message } }, status)
    const paginate = <T,>(items: T[]) => { const p = Number(url.searchParams.get('page') || 1), n = Number(url.searchParams.get('page_size') || 20); return { data: items.slice((p - 1) * n, p * n), total: items.length, page: p, page_size: n } }
    if (method !== 'GET') writes.push({ method, path, body: request.postDataJSON() ?? {} })
    if (method === 'GET' && /^\/stringers\/[^/]+\/grips$/.test(path)) return json({ data: [], total: 0, page: 1, page_size: 5 })
    if (path === '/me') return json({ id: uid(4), admin: role === 'admin' })
    if (path === '/me/stringers') return json(businesses)
    if (path === '/admin/stringer-applications') return role === 'admin' ? json(paginate(pending ? [{ ...businesses[0], id: uid(107), name: 'Pending Preview', approval_status: 'pending' }] : [])) : problem(403, 'forbidden', 'Administrator access required.')
    if (path.endsWith('/approval')) { pending = false; return json({ ...businesses[0], approval_status: 'approved' }) }
    const business = businesses.find(b => path.includes(b.id))
    if (path.endsWith('/billing')) return json({ plan: 'free', accepted_bookings: order.status === 'placed' ? 4 : 5, free_limit: 5, remaining: order.status === 'placed' ? 1 : 0, can_accept: order.status === 'placed', period_start: '2026-10-01T00:00:00+01:00', resets_at: '2026-11-01T00:00:00Z', timezone: 'Europe/Dublin', subscription_status: 'none', paid_until: null, cancel_at_period_end: false, billing_available: false, portal_available: false, monthly_price_cents: null, currency: 'EUR' })
    if(path === `/orders/${order.id}/cash-receipt`) { order.payment_status = 'paid'; order.payment_events = [{ from_status: 'unpaid', to_status: 'paid', reason: 'Cash receipt confirmed by the stringer', created_at: new Date().toISOString() }]; return json(order) }
    if (path.endsWith('/manage/profile')) return business ? json(business) : problem(404, 'not_found', 'Business not found.')
    if (path.endsWith('/manage/summary')) return json({ total_orders: business?.id === shopID ? 1 : 0, active_orders: order.status === 'placed' || order.status === 'accepted' ? 1 : 0, completed_orders: 0, cancelled_orders: order.status === 'cancelled' ? 1 : 0, unpaid_orders: order.status === 'cancelled' ? 0 : 1, booked_value_cents: order.status === 'cancelled' ? 0 : order.total_cents, currency: 'EUR' })
    if (path === `/orders/${order.id}/status`) { order.status = request.postDataJSON().status; order.events!.push({ from_status: 'placed', to_status: order.status, created_at: new Date().toISOString() }); return json(order) }
    if (path.endsWith(`/orders/${order.id}`)) return json({ ...order, customer: { first_name: 'Preview', last_name: 'Customer', phone_number: '+447700900001' } })
    if (path.endsWith('/orders')) { const status = url.searchParams.get('status'); return json(paginate(business?.id === shopID && (!status || status === order.status) ? [order] : [])) }
    if (path.endsWith('/manage/services')) return json(services)
    if (method === 'PUT' && path.includes('/services/')) { Object.assign(service, request.postDataJSON()); return json(service) }
    if (method === 'PUT' && path === `/stringers/${business?.id}`) { Object.assign(business!, request.postDataJSON()); return json(business) }
    if (path.endsWith('/inventory/lots')) { const q = (url.searchParams.get('q') ?? '').toLowerCase(); return json(paginate(lots.filter(l => `${l.brand} ${l.model}`.toLowerCase().includes(q)))) }
    if (path.endsWith('/adjustments')) { const lot = lots.find(l => path.includes(l.id))!, n = request.postDataJSON().quantity_delta; if (lot.quantity + n < lot.reserved) return problem(409, 'stock_conflict', 'Adjustment would remove reserved stock.'); lot.quantity += n; return json(lot) }
    if (method === 'POST' && path.endsWith('/lots')) { const body = request.postDataJSON(); const v = variants.find(v => path.includes(v.id))!; const lot: S['StockLot'] = { id: uid(8000 + lots.length + 1), inventory_id: v.inventory_id!, variant_id: v.id, brand: v.brand, model: v.model, gauge_mm: v.gauge_mm, colour: v.colour, sport: v.sport, reserved: 0, ...body }; lots.push(lot); return json(lot, 201) }
    if (method === 'PUT' && path.includes('/inventory/')) { const v = variants.find(v => path.endsWith(v.id))!; Object.assign(v, request.postDataJSON()); return json({ id: v.inventory_id }) }
    if (path.endsWith('/manage/inventory') || path === '/strings') {
      const q = (url.searchParams.get('q') ?? '').toLowerCase(), variantID = url.searchParams.get('variant_id')
      const source = business?.id === otherShopID ? [otherVariant] : variants
      let filtered = source.filter(v => (!variantID || v.id === variantID) && `${v.brand} ${v.model}`.toLowerCase().includes(q))
      if (path === '/strings') filtered = filtered.map(v => ({ ...v, inventory_id: undefined, material_price_cents: 0, available: false }))
      return json(paginate(filtered))
    }
    return problem(404, 'unexpected_request', path)
  })
  return { businesses, variants, order, lots, service, writes }
}
