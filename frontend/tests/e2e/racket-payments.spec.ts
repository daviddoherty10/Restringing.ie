import { openManualStrings } from './booking-controls.ts'
import { expect, test } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { dashboardAPI, shopID } from './dashboard-fixture.ts'
import { hybridsAPI } from './hybrids-fixture.ts'
import { trackingAPI } from './tracking-fixture.ts'

test('owner controls onsite cash/card and ready online payments; staff cannot edit', async ({ page }) => {
  await dashboardAPI(page)
  const options = { onsite_cash: true, onsite_card: false, online_enabled: false, online_ready: false, online_available: false, online_configured: true }
  const writes: unknown[] = []
  await page.route('**/payment-options', async route => {
    if (route.request().method() === 'PUT') { const body = route.request().postDataJSON(); writes.push(body); Object.assign(options, body) }
    return route.fulfill({ json: options })
  })
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await page.getByRole('button', { name: 'Payments', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'Pay online through Stripe', exact: true })).toBeDisabled()
  await page.getByRole('checkbox', { name: 'Card onsite using your own terminal', exact: true }).check()
  await page.getByRole('checkbox', { name: 'Cash onsite', exact: true }).uncheck()
  await page.getByRole('button', { name: 'Save payment options', exact: true }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Payment options saved' })).toBeVisible()
  expect(writes[0]).toEqual({ onsite_cash: false, onsite_card: true, online_enabled: false })
  options.online_ready = true
  await page.getByRole('button', { name: 'Refresh payment status', exact: true }).click()
  await page.getByRole('checkbox', { name: 'Pay online through Stripe', exact: true }).check()
  await page.getByRole('button', { name: 'Save payment options', exact: true }).click()
  expect(writes.at(-1)).toEqual({ onsite_cash: false, onsite_card: true, online_enabled: true })
  await page.setViewportSize({ width: 320, height: 740 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  const audit = await new AxeBuilder({ page }).include('[aria-label="Racket payment settings"]').withTags(['wcag2a','wcag2aa','wcag21aa','wcag22aa']).analyze()
  expect(audit.violations.map(v => v.id)).toEqual([])
  await page.locator('[aria-label="Racket payment settings"]').screenshot({ path: `/tmp/restringing-payment-settings-${test.info().project.name}.png` })
  await page.unrouteAll()
  await dashboardAPI(page, 'staff')
  await page.route('**/payment-options', route => route.fulfill({ json: options }))
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await page.getByRole('button', { name: 'Payments', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'Cash onsite', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Save payment options', exact: true })).toHaveCount(0)
})

test('customer choice is quoted, restored and invalidates an old quote when changed', async ({ page }) => {
  const f = await hybridsAPI(page)
  await page.route('**/payment-options', route => route.fulfill({ json: { onsite_cash: false, onsite_card: true, online_enabled: true, online_available: true, online_ready: true, online_configured: true } }))
  await page.goto(`/stringers/${shopID}/book`)
  await openManualStrings(page)
  await page.getByRole('button', { name: /Preview String 01/ }).click()
  const onsite = page.getByRole('radio', { name: 'Pay onsite', exact: true })
  const online = page.getByRole('radio', { name: 'Pay online', exact: true })
  await expect(onsite).toBeChecked()
  await expect(onsite).toHaveAccessibleDescription('Pay your stringer in person by card.')
  await expect(online).toHaveAccessibleDescription('Pay your stringer through Stripe after booking.')
  const audit = await new AxeBuilder({ page }).include('.booking-summary').withTags(['wcag2a','wcag2aa','wcag21aa','wcag22aa']).analyze()
  expect(audit.violations.map(v => v.id)).toEqual([])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.getByText('Pay your stringer through Stripe after booking.', { exact: true }).click()
  await expect(online).toBeChecked()
  await page.getByRole('button', { name: /Request quote/ }).click()
  expect(f.quotes.at(-1)?.configuration.payment_method).toBe('online')
  await expect(page.getByRole('button', { name: 'Confirm booking', exact: true })).toBeVisible()
  await online.focus()
  await page.keyboard.press('ArrowUp')
  await expect(onsite).toBeChecked()
  await expect(page.getByRole('button', { name: 'Confirm booking', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: /Request quote/ }).click()
  expect(f.quotes.at(-1)?.configuration.payment_method).toBe('onsite')
  await page.reload()
  await expect(onsite).toBeChecked()
  await expect(page.getByRole('button', { name: 'Request quote' })).toBeEnabled()

  await page.addInitScript(id => {
    const key = `restringing:booking:${id}`
    const saved = sessionStorage.getItem(key)
    if (saved) {
      sessionStorage.setItem(key, JSON.stringify({ ...JSON.parse(saved), paymentMethod: 'cash' }))
    }
  }, shopID)
  await page.reload()
  await expect(page.getByRole('alert')).toHaveText('Choose an available payment method.')
  await expect(onsite).not.toBeChecked()
  await onsite.check()
  await expect(page.getByRole('button', { name: 'Request quote' })).toBeEnabled()
})

test('onsite card receipt records payment without advancing fulfilment', async ({ page }) => {
  const f = await dashboardAPI(page)
  Object.assign(f.order, { payment_method: 'onsite', status: 'accepted' })
  Object.assign(f.order.configuration, { onsite_cash: false, onsite_card: true })
  const received: unknown[] = []
  await page.route('**/onsite-receipt', async route => { received.push(route.request().postDataJSON()); f.order.payment_status = 'paid'; return route.fulfill({ json: f.order }) })
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Record onsite payment', exact: true }).click()
  await expect(page.getByLabel('Received by')).toHaveValue('card')
  await page.getByRole('button', { name: 'Confirm onsite payment received', exact: true }).click()
  expect(received).toEqual([{ method: 'card' }]); expect(f.order.status).toBe('accepted')
  await expect(page.getByRole('button', { name: 'Record onsite payment', exact: true })).toHaveCount(0)
})

test('only an online order offers checkout and a browser return never marks it paid', async ({ page }) => {
  const f = await trackingAPI(page)
  f.order.payment_method = 'online'; f.order.payment_status = 'unpaid'
  await page.route('**/payment-checkout', route => route.fulfill({ status: 201, json: { url: 'https://checkout.stripe.com/c/pay/test' } }))
  await page.route('https://checkout.stripe.com/**', route => route.fulfill({ contentType: 'text/html', body: '<h1>Hosted payment preview</h1>' }))
  await page.goto(`/orders/${f.order.id}?payment=return`)
  await expect(page.getByText(/Online payment: unpaid/)).toBeVisible()
  await page.getByRole('button', { name: 'Pay online', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Hosted payment preview', exact: true })).toBeVisible()
  f.order.payment_method = 'onsite'; await page.goto(`/orders/${f.order.id}`)
  await expect(page.getByRole('button', { name: 'Pay online', exact: true })).toHaveCount(0)
})
