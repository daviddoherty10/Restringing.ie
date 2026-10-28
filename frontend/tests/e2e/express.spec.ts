import { expect, test, type Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
import { dashboardAPI, shopID, uid } from './dashboard-fixture.ts'
import { trackingAPI } from './tracking-fixture.ts'
type S = components['schemas']
async function expressAPI(page: Page) {
  const d = await dashboardAPI(page)
  d.service.express = true
  d.service.express_fee_cents = 799
  d.service.express_turnaround =
    'Within 24 hours of receiving your racket, after acceptance.'
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
  const quotes: S['Quote'][] = []
  await page.route('**/api/v1/**', async (route) => {
    const r = route.request(),
      url = new URL(r.url()),
      path = url.pathname.replace('/api/v1', '')
    const json = (body: unknown, status = 200) =>
      route.fulfill({ json: body, status })
    if (path === '/rackets')
      return json({ data: [racket], page: 1, page_size: 50, total: 1 })
    if (path === `/stringers/${shopID}`) return json(d.businesses[0])
    if (path.endsWith('/services') && r.method() === 'GET')
      return json([d.service])
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
        notice: 'Verified fictional model constraints.',
      })
    if (path.endsWith('/inventory'))
      return json({
        data: d.variants,
        page: 1,
        page_size: 50,
        total: d.variants.length,
      })
    if (path.endsWith('/quotes')) {
      const b = r.postDataJSON() as S['Booking']
      if (b.express && !d.service.express)
        return json(
          {
            error: {
              code: 'invalid_configuration',
              message: 'Express is not offered.',
              fields: { express: 'Choose standard turnaround.' },
            },
          },
          422,
        )
      const q: S['Quote'] = {
        id: uid(6100 + quotes.length),
        stringer_id: shopID,
        configuration: {
          ...b,
          racket_snapshot: 'Preview Tennis Model',
          string_snapshot: 'Preview String 01',
          ...(b.express
            ? { express_turnaround_snapshot: d.service.express_turnaround }
            : {}),
        },
        total_cents: 2699 + (b.express ? d.service.express_fee_cents : 0),
        currency: 'EUR',
        expires_at: new Date(Date.now() + 600000).toISOString(),
        breakdown: [
          { code: 'labour', label: 'Restringing labour', amount_cents: 1500 },
          { code: 'strings', label: 'String material', amount_cents: 1099 },
          { code: 'knots', label: 'Four-knot stringing', amount_cents: 100 },
          ...(b.express
            ? [
                {
                  code: 'express',
                  label: 'Express turnaround',
                  amount_cents: d.service.express_fee_cents,
                },
              ]
            : []),
        ],
      }
      quotes.push(q)
      return json(q, 201)
    }
    return route.fallback()
  })
  return { ...d, quotes, racket }
}
async function setupBooking(page: Page, racketID: string) {
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(racketID)
  await page.getByRole('button', { name: /Preview String 01/ }).click()
}
test('owner configures an exact express fee and turnaround while staff cannot edit it', async ({
  page,
}) => {
  const d = await dashboardAPI(page)
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Services', exact: true }).click()
  await page
    .getByRole('checkbox', { name: 'Offer express turnaround', exact: true })
    .check()
  await page
    .getByRole('textbox', {
      name: 'Express turnaround surcharge (€)',
      exact: true,
    })
    .fill('7.99')
  await page
    .getByRole('textbox', {
      name: 'Express turnaround description',
      exact: true,
    })
    .fill('Within 24 hours of receiving the racket.')
  await page
    .getByRole('button', { name: 'Save service settings', exact: true })
    .click()
  await expect(page.getByRole('status').first()).toContainText(
    'Service settings saved.',
  )
  const write = d.writes.find(
    (w) => w.method === 'PUT' && w.path.includes('/services/'),
  )!
  expect(write.body.express).toBe(true)
  expect(write.body.express_fee_cents).toBe(799)
  expect(write.body.express_turnaround).toBe(
    'Within 24 hours of receiving the racket.',
  )
  await page.unrouteAll()
  await dashboardAPI(page, 'staff')
  await page.reload()
  await page.getByRole('button', { name: 'Services', exact: true }).click()
  await expect(
    page.getByRole('checkbox', {
      name: 'Offer express turnaround',
      exact: true,
    }),
  ).toBeDisabled()
  await expect(
    page.getByRole('textbox', {
      name: 'Express turnaround surcharge (€)',
      exact: true,
    }),
  ).toBeDisabled()
})
test('standard is the default and selecting express requires a fresh itemized quote', async ({
  page,
}) => {
  const d = await expressAPI(page)
  await setupBooking(page, d.racket.id)
  await expect(
    page.getByRole('radio', { name: 'Standard turnaround', exact: true }),
  ).toBeChecked()
  await page
    .getByRole('button', { name: 'Request server quote', exact: false })
    .click()
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toBeVisible()
  expect(d.quotes[0].configuration.express).toBe(false)
  expect(d.quotes[0].total_cents).toBe(2699)
  await page
    .getByRole('radio', { name: 'Express turnaround', exact: false })
    .check()
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toHaveCount(0)
  await page
    .getByRole('button', { name: 'Request server quote', exact: false })
    .click()
  await expect(page.locator('.booking-express-quoted')).toContainText(
    d.service.express_turnaround,
  )
  await expect(
    page
      .locator('.booking-reviewed .booking-summary-line')
      .filter({ hasText: 'Express turnaround' }),
  ).toContainText('€7.99')
  expect(d.quotes[1].configuration.express).toBe(true)
  expect(d.quotes[1].total_cents).toBe(3498)
  await page.reload()
  await expect(
    page.getByRole('radio', { name: 'Express turnaround', exact: false }),
  ).toBeChecked()
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toHaveCount(0)
})
test('free express remains selectable and an unavailable saved express choice needs review', async ({
  page,
}) => {
  const d = await expressAPI(page)
  d.service.express_fee_cents = 0
  await setupBooking(page, d.racket.id)
  await page
    .getByRole('radio', { name: 'Express turnaround', exact: false })
    .check()
  await page
    .getByRole('button', { name: 'Request server quote', exact: false })
    .click()
  await expect(
    page
      .locator('.booking-reviewed .booking-summary-line')
      .filter({ hasText: 'Express turnaround' }),
  ).toContainText('€0.00')
  expect(d.quotes[0].configuration.express).toBe(true)
  d.service.express = false
  await page.reload()
  await expect(page.getByRole('alert')).toContainText(
    'Express is no longer offered.',
  )
  await expect(
    page.getByRole('button', { name: 'Request server quote', exact: false }),
  ).toBeDisabled()
  await page
    .getByRole('radio', { name: 'Standard turnaround', exact: true })
    .check()
  await expect(
    page.getByRole('button', { name: 'Request server quote', exact: false }),
  ).toBeEnabled()
})
test('profile advertises the optional fee and is responsive on a narrow screen', async ({
  page,
}) => {
  const d = await expressAPI(page)
  await page.setViewportSize({ width: 320, height: 700 })
  await page.goto(`/stringers/${shopID}`)
  await expect(page.locator('.profile-express')).toContainText(
    'Express available: +€7.99',
  )
  await expect(page.locator('.profile-express')).toContainText(
    d.service.express_turnaround,
  )
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
})
test('tracker shows the originally booked express terms separately from the ready estimate', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  f.order.configuration.express = true
  f.order.configuration.express_turnaround_snapshot =
    'Original promise: within 24 hours of receipt.'
  f.order.breakdown.push({
    code: 'express',
    label: 'Express turnaround',
    amount_cents: 799,
  })
  f.order.total_cents += 799
  await page.goto(`/orders/${f.order.id}`)
  await expect(page.locator('.tracking-hero .tracking-express')).toContainText(
    'Original promise: within 24 hours of receipt.',
  )
  await expect(
    page.getByText('Awaiting your stringer’s estimate'),
  ).toBeVisible()
  await expect(
    page.getByRole('region', { name: 'Booking & payment' }),
  ).toContainText('€7.99')
})
