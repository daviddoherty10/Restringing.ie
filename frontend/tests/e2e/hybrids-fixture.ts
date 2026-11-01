import type { Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
import { dashboardAPI, shopID, uid } from './dashboard-fixture.ts'
type S = components['schemas']
export async function hybridsAPI(page: Page) {
  const d = await dashboardAPI(page)
  d.service.hybrid = true
  d.variants.forEach((v, i) => {
    v.mains_price_cents = i === 0 ? 0 : 590 + i
    v.crosses_price_cents = 725 + i
    v.max_prestretch_percent = i === 1 ? 0 : 10
  })
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
    mains_length_mm: 6400,
    crosses_length_mm: 5600,
    hybrid_technical_source: 'Fictional separate lengths',
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
    if (path.endsWith('/hybrid-inventory')) {
      const q = url.searchParams.get('q')?.toLowerCase() ?? '',
        id = url.searchParams.get('variant_id'),
        component = url.searchParams.get('component'),
        material = url.searchParams.get('material')
      const data = d.variants.filter(
        (v) =>
          v.id !== soldOut &&
          (!id || id === v.id) &&
          (!q || `${v.brand} ${v.model}`.toLowerCase().includes(q)) &&
          (!material || material === v.material) &&
          (component === 'mains'
            ? v.mains_price_cents != null
            : v.crosses_price_cents != null),
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
    if (path.endsWith('/hybrid-prices') && req.method() === 'PUT') {
      const body = req.postDataJSON() as S['HybridPrices']
      const v = d.variants.find((v) => path.includes(v.id))!
      v.mains_price_cents = body.mains_price_cents ?? undefined
      v.crosses_price_cents = body.crosses_price_cents ?? undefined
      requests.push({ body, key: undefined })
      return json(body)
    }
    if (path.endsWith('/quotes')) {
      const b = req.postDataJSON() as S['Booking'],
        m = d.variants.find((v) => v.id === b.variant_id)!,
        c = d.variants.find((v) => v.id === b.crosses_variant_id)
      const breakdown: S['PriceLine'][] = [
        { code: 'labour', label: 'Restringing labour', amount_cents: 1500 },
        { code: 'knots', label: 'Four-knot stringing', amount_cents: 100 },
        ...(b.hybrid
          ? [
              {
                code: 'mains_strings',
                label: 'Mains string material',
                amount_cents: m.mains_price_cents!,
              },
              {
                code: 'crosses_strings',
                label: 'Crosses string material',
                amount_cents: c!.crosses_price_cents!,
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
                mains_snapshot: `${m.brand} ${m.model} ${m.gauge_mm} mm ${m.colour}`,
                crosses_snapshot: `${c!.brand} ${c!.model} ${c!.gauge_mm} mm ${c!.colour}`,
                mains_length_mm: 6400,
                crosses_length_mm: 5600,
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
