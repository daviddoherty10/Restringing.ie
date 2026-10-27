import type { Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
type S = components['schemas']
export const catalogueID = (n: number) => `91000000-0000-4000-8000-${String(n).padStart(12, '0')}`
export async function catalogueAPI(page: Page, admin = true) {
  const products: S['Product'][] = Array.from({ length: 24 }, (_, i) => ({ id: catalogueID(1001 + i), brand: 'Preview', model: `String ${String(i + 1).padStart(2, '0')}`, material: 'multifilament', sport: 'tennis', description: 'Fictional catalogue item', comfort: 4, control: 3, power: 3, spin: 3, durability: 3, max_prestretch_percent: 10, active: i % 2 === 0 }))
  const variants: S['CatalogueVariant'][] = products.map((p, i) => ({ id: catalogueID(2001 + i), product_id: p.id, brand: p.brand, model: p.model, material: p.material, sport: p.sport, gauge_mm: 1.25, colour: 'Natural', active: true, product_active: p.active }))
  const rackets: S['Racket'][] = Array.from({ length: 22 }, (_, i) => ({ id: catalogueID(3001 + i), brand: 'Preview', model: `Racket ${String(i + 1).padStart(2, '0')}`, sport: 'tennis', min_tension_centi_kg: 1800, max_tension_centi_kg: 3000, step_centi_kg: 50, max_difference_centi_kg: 200, allowed_knots: [2, 4], different_tensions_require_four_knots: true, string_length_mm: 12000, technical_source: 'Fictional preview source', verified: i % 2 === 0 }))
  const writes: { method: string; path: string; body: Record<string, unknown> }[] = [], reads: string[] = []
  await page.route('**/api/v1/**', async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname.replace('/api/v1', ''), method = request.method()
    const json = (value: unknown, status = 200) => route.fulfill({ status, json: value })
    if (method === 'GET') reads.push(path)
    else writes.push({ method, path, body: request.postDataJSON() ?? {} })
    if (path === '/me') return json({ id: catalogueID(1), admin })
    if (!admin) return json({ error: { code: 'forbidden', message: 'Administrator access required.' } }, 403)
    const paginate = <T,>(data: T[]) => { const n = Number(url.searchParams.get('page_size') || 20), p = Number(url.searchParams.get('page') || 1); return { data: data.slice((p - 1) * n, p * n), total: data.length, page: p, page_size: n } }
    const q = (url.searchParams.get('q') || '').toLowerCase(), sport = url.searchParams.get('sport'), material = url.searchParams.get('material'), active = url.searchParams.get('active'), verified = url.searchParams.get('verified'), productID = url.searchParams.get('product_id')
    if (path === '/admin/string-products' && method === 'GET') return json(paginate(products.filter(p => `${p.brand} ${p.model}`.toLowerCase().includes(q) && (!sport || p.sport === sport) && (!material || p.material === material) && (!active || String(p.active) === active))))
    if (path === '/admin/string-products' && method === 'POST') { const value = { ...request.postDataJSON(), id: catalogueID(10001 + products.length) } as S['Product']; products.push(value); return json(value, 201) }
    if (path.startsWith('/admin/string-products/')) { const value = products.find(p => path.endsWith(p.id))!; if (method === 'PUT') Object.assign(value, request.postDataJSON()); return json(value) }
    const joined = variants.map(v => { const p = products.find(p => p.id === v.product_id)!; return { ...v, brand: p.brand, model: p.model, sport: p.sport, material: p.material, product_active: p.active } })
    if (path === '/admin/string-variants' && method === 'GET') return json(paginate(joined.filter(v => `${v.brand} ${v.model} ${v.colour} ${v.gauge_mm}`.toLowerCase().includes(q) && (!productID || v.product_id === productID) && (!sport || v.sport === sport) && (!active || String(v.active) === active))))
    if (path === '/admin/string-variants' && method === 'POST') { const input = request.postDataJSON(), p = products.find(p => p.id === input.product_id)!; const value = { brand: p.brand, model: p.model, sport: p.sport, material: p.material, product_active: p.active, ...input, id: catalogueID(20001 + variants.length) } as S['CatalogueVariant']; variants.push(value); return json({ id: value.id }, 201) }
    if (path.startsWith('/admin/string-variants/')) { const value = variants.find(v => path.endsWith(v.id))!; if (method === 'PUT') Object.assign(value, request.postDataJSON()); return json(method === 'PUT' ? { id: value.id } : joined.find(v => v.id === value.id)) }
    if (path === '/admin/rackets' && method === 'GET') return json(paginate(rackets.filter(r => `${r.brand} ${r.model}`.toLowerCase().includes(q) && (!sport || r.sport === sport) && (!verified || String(r.verified) === verified))))
    if (path === '/admin/rackets' && method === 'POST') { const value = { ...request.postDataJSON(), id: catalogueID(30001 + rackets.length) } as S['Racket']; rackets.push(value); return json(value, 201) }
    if (path.startsWith('/admin/rackets/')) { const value = rackets.find(r => path.endsWith(r.id))!; if (method === 'PUT') Object.assign(value, request.postDataJSON()); return json(value) }
    return json({ error: { code: 'unexpected_request', message: path } }, 404)
  })
  return { products, variants, rackets, reads, writes }
}
