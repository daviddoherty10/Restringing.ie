import { expect, test, type Page } from '@playwright/test'
const shop = '90000000-0000-4000-8000-000000000101'
const model = '90000000-0000-4000-8000-000000003001'
const variant = '90000000-0000-4000-8000-000000002001'
const bookPath = `/stringers/${shop}/book`
async function mockAPI(page: Page, available = true) {
  let signedIn = false
  let verifications = 0
  let sentPhone = ''
  const service = { active: true, own_strings: false, stencil: true, stencil_cents: 300, different_tensions: true, prestretch_options: { '5': 300 } }
  await page.route('**/api/v1/**', async route => {
    const req = route.request(), path = new URL(req.url()).pathname.replace('/api/v1', '')
    const json = (data: unknown, status = 200, headers?: Record<string, string>) => route.fulfill({ status, json: data, headers })
    if (path === '/auth/config') return json({ available, allowed_calling_codes: ['+353', '+44'], resend_after_seconds: 60 })
    if (path === '/auth/phone/request') { sentPhone = req.postDataJSON().phone_number; return json({ challenge_id: 'opaque-test-challenge', expires_at: new Date(Date.now() + 600000).toISOString(), resend_after_seconds: 60 }, 201) }
    if (path === '/auth/phone/verify') {
      verifications++
      if (req.postDataJSON().code !== '731946') return json({ error: { code: 'invalid_verification', message: 'Code invalid or expired.' } }, 400)
      signedIn = true; return json({ id: 'customer', admin: false })
    }
    if (path === '/auth/logout') { signedIn = false; return json({ signed_out: true }) }
    if (path === '/me') return signedIn ? json({ id: 'customer', admin: false }) : json({ error: { code: 'unauthorized', message: 'Sign in required.' } }, 401)
    if (path === '/orders') return json({ data: [], page: 1, page_size: 10, total: 0 })
    if (path === '/rackets') return json({ data: [{ id: model, brand: 'Preview', model: 'Tennis model', sport: 'tennis', verified: true }], total: 1, page: 1, page_size: 50 })
    if (path.endsWith('/booking-configuration')) return json({ service, min_tension_centi_kg: 1800, max_tension_centi_kg: 3000, step_centi_kg: 50, max_difference_centi_kg: 200, allowed_knots: [2, 4], notice: 'Fictional fixture constraints' })
    if (path.endsWith('/inventory')) return json({ data: [{ id: variant, brand: 'Preview', model: 'Soft Multi', sport: 'tennis', material: 'multifilament', gauge_mm: 1.3, colour: 'Natural', material_price_cents: 1000, max_prestretch_percent: 10 }], total: 1, page: 1, page_size: 10 })
    if (path.endsWith('/quotes')) {
      if (!signedIn) return json({ error: { code: 'unauthorized', message: 'Sign in required.' } }, 401)
      return json({ id: 'quote', total_cents: 2500, currency: 'EUR', breakdown: [{ code: 'labour', label: 'Labour', amount_cents: 1500 }, { code: 'string', label: 'Strings', amount_cents: 1000 }], expires_at: new Date(Date.now() + 600000).toISOString() }, 201)
    }
    return json({ error: { code: 'unexpected_test_request', message: path } }, 404)
  })
  return { phone: () => sentPhone, checks: () => verifications }
}
async function signIn(page: Page) {
  await page.getByLabel('Mobile number').fill('087 123 4567')
  await page.getByRole('button', { name: 'Send sign-in code' }).click()
  await expect(page.getByRole('heading', { name: 'Check your phone.' })).toBeVisible()
  await expect(page.getByRole('button', { name: /Resend in/ })).toBeDisabled()
  await page.getByLabel('One-time code').fill('000000')
  await page.getByRole('button', { name: 'Sign in →', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveText('Code invalid or expired.')
  await page.getByLabel('One-time code').fill('731946')
  await page.getByRole('button', { name: 'Sign in →', exact: true }).click()
}
test('phone sign-in, invalid code, safe return and logout', async ({ page }) => {
  const api = await mockAPI(page)
  await page.goto('/signin?returnTo=https://evil.example')
  await signIn(page)
  await expect(page).toHaveURL('/orders')
  expect(api.phone()).toBe('+353871234567')
  expect(api.checks()).toBe(2)
  await expect(page.getByRole('link', { name: 'My orders' })).toBeVisible()
  await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await expect(page).toHaveURL('/')
  await expect(page.getByRole('link', { name: 'Sign in ↗' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})
test('booking setup survives sign-in and requires a fresh server quote', async ({ page }) => {
  await mockAPI(page)
  await page.goto(bookPath)
  await page.getByRole('combobox', { name: 'Racket model', exact: true }).selectOption(model)
  await page.getByRole('button', { name: /Preview Soft Multi/ }).click()
  await page.getByLabel('Mains tension (kg)').fill('24.5')
  await page.getByLabel('Additional instructions').fill('Keep this setup')
  await page.getByRole('button', { name: 'Request server quote' }).click()
  await expect(page).toHaveURL(/\/signin\?returnTo=/)
  await signIn(page)
  await expect(page).toHaveURL(bookPath)
  await expect(page.getByRole('combobox', { name: 'Racket model', exact: true })).toHaveValue(model)
  await expect(page.getByLabel('Mains tension (kg)')).toHaveValue('24.5')
  await expect(page.getByLabel('Additional instructions')).toHaveValue('Keep this setup')
  await expect(page.getByText('Selected: Preview Soft Multi')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Place unpaid booking' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Request server quote' }).click()
  await expect(page.getByRole('button', { name: 'Place unpaid booking' })).toBeVisible()
})
test('SMS disabled and rate-limited states are actionable', async ({ page }) => {
  await mockAPI(page, false)
  await page.goto('/signin')
  await expect(page.getByRole('status')).toContainText('SMS sign-in is temporarily unavailable')
  await expect(page.getByLabel('Mobile number')).toHaveCount(0)
  await page.unrouteAll()
  await mockAPI(page)
  await page.route('**/auth/phone/request', route => route.fulfill({ status: 429, headers: { 'Retry-After': '120' }, json: { error: { code: 'rate_limited', message: 'Too many attempts. Please wait.' } } }))
  await page.reload()
  await page.getByLabel('Mobile number').fill('0871234567')
  await page.getByRole('button', { name: 'Send sign-in code' }).click()
  await expect(page.getByRole('alert')).toContainText('Too many attempts')
  await expect(page.getByRole('button', { name: /Try again in 1[12][0-9]s/ })).toBeDisabled()
})
test('uncertain booking retry keeps the same quote and key after a reload', async ({ page }) => {
  await mockAPI(page)
  await page.goto('/signin?returnTo=' + encodeURIComponent(bookPath))
  await signIn(page)
  await expect(page).toHaveURL(bookPath)
  await page.getByRole('combobox', { name: 'Racket model', exact: true }).selectOption(model)
  await page.getByRole('button', { name: /Preview Soft Multi/ }).click()
  await page.getByLabel('Mains tension (kg)').fill('24.5')
  await page.getByRole('button', { name: 'Request server quote' }).click()
  const attempts: { key: string | undefined; quote: string }[] = []
  await page.route('**/api/v1/orders', async route => {
    const r = route.request()
    attempts.push({ key: r.headers()['idempotency-key'], quote: r.postDataJSON().quote_id })
    if (attempts.length === 1) return route.abort('failed')
    return route.fulfill({ status: 200, json: { id: shop, payment_status: 'unpaid', status: 'placed' } })
  })
  await page.getByRole('button', { name: 'Place unpaid booking' }).click()
  await expect(page.getByRole('button', { name: 'Retry same order request' })).toBeVisible()
  await page.reload()
  await page.getByRole('button', { name: 'Retry pending booking' }).click()
  await expect(page.getByRole('status')).toContainText('Booking created')
  expect(attempts).toHaveLength(2)
  expect(attempts[0].key).toBeTruthy()
  expect(attempts[1]).toEqual(attempts[0])
})
