import type { Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
import { dashboardAPI, shopID, otherShopID, uid } from './dashboard-fixture.ts'
type S = components['schemas']
export async function gripsAPI(
  page: Page,
  role: 'owner' | 'staff' | 'admin' | 'customer' = 'owner',
) {
  const base = await dashboardAPI(page, role === 'customer' ? 'owner' : role)
  const products: S['GripProduct'][] = Array.from({ length: 24 }, (_, i) => ({
    id: uid(101001 + i),
    brand: 'Preview',
    model: `Grip ${String(i + 1).padStart(2, '0')}`,
    kind: i % 2 ? 'replacement_grip' : 'overgrip',
    colour: i % 2 ? 'Black' : 'White',
    description: 'Fictional grip fixture.',
    active: i !== 23,
  }))
  const stocks = new Map<string, S['GripStock']>()
  for (let i = 0; i < 12; i++)
    stocks.set(`${shopID}:${products[i].id}`, {
      ...products[i],
      inventory_id: uid(102001 + i),
      fitted_price_cents: i === 1 ? 0 : 599 + i,
      available: true,
      quantity: i === 0 ? 8 : 3,
      reserved: i === 0 ? 2 : 0,
    })
  stocks.set(`${otherShopID}:${products[22].id}`, {
    ...products[22],
    model: 'Other business grip',
    inventory_id: uid(102099),
    fitted_price_cents: 1500,
    available: true,
    quantity: 2,
    reserved: 0,
  })
  const movements: S['GripMovement'][] = []
  const writes: {
    path: string
    body: Record<string, unknown>
    key: string | undefined
  }[] = []
  const keys = new Map<string, unknown>()
  const quotes: S['Quote'][] = []
  let failStock = false,
    failCatalogue = false
  const racket: S['Racket'] = {
    id: uid(7001),
    brand: 'Preview',
    model: 'Tennis Model',
    sport: 'tennis',
    verified: true,
    min_tension_centi_kg: 1800,
    max_tension_centi_kg: 3000,
    step_centi_kg: 50,
    max_difference_centi_kg: 200,
    allowed_knots: [2, 4],
    different_tensions_require_four_knots: true,
    string_length_mm: 12000,
    technical_source: 'Fictional fixture',
  }
  await page.route('**/api/v1/**', async (route) => {
    const req = route.request(),
      url = new URL(req.url()),
      path = url.pathname.replace('/api/v1', ''),
      method = req.method()
    const json = (body: unknown, status = 200) =>
      route.fulfill({ json: body, status })
    const error = (code: string, message: string, status = 409) =>
      json({ error: { code, message } }, status)
    const paginate = <T>(items: T[]) => {
      const p = Number(url.searchParams.get('page') || 1),
        size = Number(url.searchParams.get('page_size') || 20)
      return {
        data: items.slice((p - 1) * size, p * size),
        page: p,
        page_size: size,
        total: items.length,
      }
    }
    const filter = (items: S['GripProduct'][]) =>
      items.filter(
        (g) =>
          (!url.searchParams.get('q') ||
            `${g.brand} ${g.model} ${g.colour}`
              .toLowerCase()
              .includes(url.searchParams.get('q')!.toLowerCase())) &&
          (!url.searchParams.get('kind') ||
            g.kind === url.searchParams.get('kind')) &&
          (!url.searchParams.get('grip_product_id') ||
            g.id === url.searchParams.get('grip_product_id')),
      )
    if (method !== 'GET')
      writes.push({
        path,
        body: req.postDataJSON() ?? {},
        key: req.headers()['idempotency-key'],
      })
    if (path === '/me')
      return json({
        id: uid(role === 'customer' ? 1 : 4),
        admin: role === 'admin',
      })
    if (path === '/admin/grips' && method === 'GET') {
      if (role !== 'admin') return error('forbidden', 'Admin required.', 403)
      const active = url.searchParams.get('active')
      return json(
        paginate(
          filter(products).filter(
            (g) => !active || g.active === (active === 'true'),
          ),
        ),
      )
    }
    if (path === '/admin/grips' && method === 'POST') {
      const key = req.headers()['idempotency-key']!
      if (keys.has(key)) return json(keys.get(key), 201)
      const p: S['GripProduct'] = {
        id: uid(101100 + products.length),
        ...req.postDataJSON(),
      }
      products.push(p)
      keys.set(key, p)
      if (failCatalogue) {
        failCatalogue = false
        return route.abort('failed')
      }
      return json(p, 201)
    }
    if (path.startsWith('/admin/grips/') && method === 'PUT') {
      const p = products.find((p) => path.endsWith(p.id))!
      Object.assign(p, req.postDataJSON())
      return json(p)
    }
    if (path === '/grips')
      return json(paginate(filter(products.filter((g) => g.active))))
    if (path.includes('/grips') || path.endsWith('/manage/grips')) {
      const business = path.split('/')[2],
        productID = path.split('/grips/')[1]?.split('/')[0],
        inventory = stocks.get(`${business}:${productID}`)
      if (path.endsWith('/stock-events') && method === 'GET')
        return json(
          paginate(
            movements
              .filter((m) => m.inventory_id === inventory?.inventory_id)
              .slice()
              .reverse(),
          ),
        )
      if (path.endsWith('/stock-events') && method === 'POST') {
        const key = req.headers()['idempotency-key']!
        if (keys.has(key)) return json(keys.get(key))
        const b = req.postDataJSON()
        if (
          !inventory ||
          inventory.quantity + b.quantity_delta < inventory.reserved
        )
          return error(
            'stock_conflict',
            'The correction would remove reserved units or make stock negative.',
          )
        inventory.quantity += b.quantity_delta
        const m: S['GripMovement'] = {
          id: uid(103001 + movements.length),
          inventory_id: inventory.inventory_id,
          quantity_delta: b.quantity_delta,
          reserved_delta: 0,
          reason: b.reason,
          order_id: null,
          created_at: new Date().toISOString(),
        }
        movements.push(m)
        keys.set(key, m)
        if (failStock) {
          failStock = false
          return route.abort('failed')
        }
        return json(m)
      }
      if (method === 'PUT') {
        const product = products.find((p) => p.id === productID)!
        const g = inventory ?? {
          ...product,
          inventory_id: uid(102100 + stocks.size),
          quantity: 0,
          reserved: 0,
          fitted_price_cents: 0,
          available: false,
        }
        Object.assign(g, req.postDataJSON())
        stocks.set(`${business}:${productID}`, g)
        return json(g)
      }
      const all = [...stocks.entries()]
        .filter(([key]) => key.startsWith(business + ':'))
        .map(([, g]) => ({
          ...g,
          active: products.find((p) => p.id === g.id)!.active,
        }))
      return json(
        paginate(
          filter(
            path.includes('/manage/grips')
              ? all
              : all.filter(
                  (g) => g.active && g.available && g.quantity > g.reserved,
                ),
          ),
        ),
      )
    }
    if (path === '/rackets')
      return json({ data: [racket], page: 1, page_size: 50, total: 1 })
    if (path.endsWith('/booking-configuration'))
      return json({
        stringer_id: shopID,
        racket,
        service: base.service,
        min_tension_centi_kg: 1800,
        max_tension_centi_kg: 3000,
        step_centi_kg: 50,
        max_difference_centi_kg: 200,
        allowed_knots: [2, 4],
        notice: 'Fictional model constraints.',
      })
    if (path.endsWith('/inventory')) return json(paginate(base.variants))
    if (path.endsWith('/quotes')) {
      const b = req.postDataJSON() as S['Booking'],
        grip = b.grip_product_id
          ? stocks.get(`${shopID}:${b.grip_product_id}`)
          : undefined
      const q: S['Quote'] = {
        id: uid(6100 + quotes.length),
        stringer_id: shopID,
        configuration: {
          ...base.order.configuration,
          ...b,
          ...(grip
            ? {
                grip_snapshot: `${grip.brand} ${grip.model} · ${grip.kind.replaceAll('_', ' ')} · ${grip.colour}`,
              }
            : {}),
        },
        breakdown: [
          { code: 'labour', label: 'Restringing labour', amount_cents: 1500 },
          { code: 'strings', label: 'String material', amount_cents: 1099 },
          { code: 'knots', label: 'Four-knot stringing', amount_cents: 100 },
          ...(grip
            ? [
                {
                  code: 'grip',
                  label: 'Grip including fitting',
                  amount_cents: grip.fitted_price_cents,
                },
              ]
            : []),
        ],
        total_cents: 2699 + (grip?.fitted_price_cents ?? 0),
        currency: 'EUR',
        expires_at: new Date(Date.now() + 600000).toISOString(),
      }
      quotes.push(q)
      return json(q, 201)
    }
    if (path === '/orders' && method === 'POST') {
      const q = quotes.find((q) => q.id === req.postDataJSON().quote_id)!
      base.order.configuration = q.configuration
      base.order.breakdown = q.breakdown
      base.order.total_cents = q.total_cents
      return json(base.order, 201)
    }
    if (path === `/orders/${base.order.id}/tracking`)
      return json({
        requested_collection_date: null,
        confirmed_ready_at: null,
        collection_instructions: '',
        issue_open: false,
        version: 0,
        stringer_name: 'Preview Baseline',
        area: 'Dublin',
        county: 'Dublin',
        can_manage: role !== 'customer',
        is_customer: role === 'customer',
        timezone: 'Europe/Dublin',
      })
    if (path === `/orders/${base.order.id}/tracking-updates`)
      return json(paginate([]))
    if (path === `/orders/${base.order.id}/messages`)
      return json({ ...paginate([]), unread_count: 0 })
    return route.fallback()
  })
  return {
    ...base,
    products,
    stocks,
    movements,
    writes,
    quotes,
    racket,
    failNextStock: () => {
      failStock = true
    },
    failNextCatalogue: () => {
      failCatalogue = true
    },
  }
}
