import type { Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
import { dashboardAPI, shopID, uid } from './dashboard-fixture.ts'
type S = components['schemas']
export async function hybridsAPI(page: Page) {
  const d = await dashboardAPI(page)
  d.service.hybrid = true
  d.variants.forEach((v, i) => {
    v.material_price_cents = i === 0 ? 0 : 1450 + i * 2
    v.max_prestretch_percent = i === 1 ? 0 : 10
  })
  const customerOnly: S['Variant'] = {
    ...d.variants[0],
    id: uid(2990),
    model: 'Customer Only String',
    inventory_id: undefined,
    available: false,
    track_stock: false,
  }
  const catalogue = [...d.variants, customerOnly]
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
    technical_source: 'Fictional browser fixture',
  }
  const quotes: S['Quote'][] = [],
    requests: { body: Record<string, unknown>; key: string | undefined }[] = []
  let failOrder = false,
    order: S['Order'] | null = null,
    soldOut = ''
  await page.route('**/api/v1/**', async (route) => {
    const req = route.request(),
      url = new URL(req.url()),
      path = url.pathname.replace('/api/v1', '')
    const json = (body: unknown, status = 200) =>
      route.fulfill({ json: body, status })
    if (path === '/rackets')
      return json({ data: [racket], page: 1, page_size: 50, total: 1 })
    if (
      path.endsWith('/inventory') &&
      req.method() === 'GET' &&
      !path.includes('/manage/')
    )
      return json({
        data: d.variants,
        page: 1,
        page_size: 10,
        total: d.variants.length,
      })
    if (path.endsWith('/booking-configuration'))
      return json({
        stringer_id: shopID,
        racket,
        service: d.service,
        min_tension_centi_kg: 1800,
        max_tension_centi_kg: 3000,
        step_centi_kg: 50,
        max_difference_centi_kg: 200,
        allowed_knots: [2, 4],
        notice: 'Fictional verified constraints.',
      })
    if (path.endsWith('/hybrid-inventory') || path === '/strings') {
      const q = url.searchParams.get('q')?.toLowerCase() ?? '',
        id = url.searchParams.get('variant_id'),
        material = url.searchParams.get('material')
      const data = (path === '/strings' ? catalogue : d.variants).filter(
        (v) =>
          (path === '/strings' || v.id !== soldOut) &&
          (!id || id === v.id) &&
          (!q || `${v.brand} ${v.model}`.toLowerCase().includes(q)) &&
          (!material || material === v.material),
      )
      const p = Number(url.searchParams.get('page') || 1),
        n = Number(url.searchParams.get('page_size') || 5)
      return json({
        data: data.slice((p - 1) * n, p * n),
        total: data.length,
        page: p,
        page_size: n,
      })
    }
    if (path.endsWith('/quotes')) {
      const b = req.postDataJSON() as S['Booking'],
        m = catalogue.find((v) => v.id === b.variant_id)!,
        c = catalogue.find((v) => v.id === b.crosses_variant_id)
      const breakdown: S['PriceLine'][] = [
        { code: 'labour', label: 'Restringing labour', amount_cents: 1500 },
        { code: 'knots', label: 'Four-knot stringing', amount_cents: 100 },
        ...(b.string_source === 'customer'
          ? [
              {
                code: 'own_strings',
                label: 'Customer-supplied strings handling',
                amount_cents: 0,
              },
            ]
          : b.hybrid
            ? [
                {
                  code: 'mains_strings',
                  label: 'Mains string material (half bed)',
                  amount_cents: Math.ceil(m.material_price_cents / 2),
                },
                {
                  code: 'crosses_strings',
                  label: 'Crosses string material (half bed)',
                  amount_cents:
                    Math.ceil(
                      (m.material_price_cents + c!.material_price_cents) / 2,
                    ) - Math.ceil(m.material_price_cents / 2),
                },
              ]
            : [
                {
                  code: 'strings',
                  label: 'String material',
                  amount_cents: m.material_price_cents,
                },
              ]),
      ]
      const q: S['Quote'] = {
        id: uid(6100 + quotes.length),
        stringer_id: shopID,
        configuration: {
          ...b,
          racket_snapshot: 'Preview Tennis Model',
          string_snapshot: b.hybrid
            ? 'Hybrid: mains and crosses'
            : 'Preview String 01',
          ...(b.hybrid
            ? {
                mains_snapshot: m
                  ? `${m.brand} ${m.model} ${m.gauge_mm} mm ${m.colour}`
                  : b.customer_mains_description!,
                crosses_snapshot: c
                  ? `${c.brand} ${c.model} ${c.gauge_mm} mm ${c.colour}`
                  : b.customer_crosses_description!,
              }
            : {}),
        },
        breakdown,
        total_cents: breakdown.reduce((sum, l) => sum + l.amount_cents, 0),
        currency: 'EUR',
        expires_at: new Date(Date.now() + 600000).toISOString(),
      }
      quotes.push(q)
      return json(q, 201)
    }
    if (path === '/orders' && req.method() === 'POST') {
      const body = req.postDataJSON(),
        key = req.headers()['idempotency-key']
      requests.push({ body, key })
      const q = quotes.find((q) => q.id === body.quote_id)!
      order ??= {
        ...d.order,
        ...q,
        id: uid(5999),
        quote_id: q.id,
        status: 'placed',
      }
      if (failOrder) {
        failOrder = false
        return json(
          {
            error: {
              code: 'unavailable',
              message: 'Connection interrupted. Retry the same booking.',
            },
          },
          503,
        )
      }
      return json(order, 201)
    }
    if (path === `/orders/${uid(5999)}`) return json(order)
    return route.fallback()
  })
  return {
    ...d,
    customerOnly,
    racket,
    quotes,
    requests,
    failNextOrder: () => {
      failOrder = true
    },
    sellOut: (id: string) => {
      soldOut = id
    },
  }
}
