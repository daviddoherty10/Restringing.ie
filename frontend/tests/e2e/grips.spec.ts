import { expect, test, type Page } from '@playwright/test'
import { gripsAPI } from './grips-fixture.ts'
import { shopID, otherShopID } from './dashboard-fixture.ts'
async function book(page: Page, racket: string) {
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(racket)
  await page.getByRole('button', { name: /Preview String 01/ }).click()
}
test('admin creates a grip with a safe retry, then activates it in the shared catalogue', async ({
  page,
}) => {
  const f = await gripsAPI(page, 'admin')
  f.failNextCatalogue()
  await page.goto('/admin/catalogue')
  await page.getByRole('button', { name: 'Grips', exact: true }).click()
  await page.getByRole('button', { name: 'Create grip', exact: true }).click()
  await expect(
    page.getByRole('checkbox', {
      name: 'Active in the shared grip catalogue',
      exact: true,
    }),
  ).not.toBeChecked()
  await page.getByLabel('Grip brand', { exact: true }).fill('New Preview')
  await page.getByLabel('Grip model', { exact: true }).fill('Soft Contact')
  await page.getByLabel('Grip colour', { exact: true }).fill('Blue')
  await page
    .getByRole('button', { name: 'Save grip product', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Retry same grip save', exact: true }),
  ).toBeVisible()
  await expect(page.getByLabel('Grip model', { exact: true })).toBeDisabled()
  await page
    .getByRole('button', { name: 'Retry same grip save', exact: true })
    .click()
  await page
    .getByLabel('Search grip products', { exact: true })
    .fill('Soft Contact')
  await expect(
    page.getByRole('heading', { name: /New Preview Soft Contact/ }),
  ).toBeVisible()
  expect(f.products.filter((p) => p.brand === 'New Preview')).toHaveLength(1)
  const calls = f.writes.filter((w) => w.path === '/admin/grips')
  expect(calls).toHaveLength(2)
  expect(calls[1]).toEqual(calls[0])
  await page
    .getByRole('button', { name: /Edit grip New Preview Soft Contact/ })
    .click()
  await page
    .getByRole('checkbox', {
      name: 'Active in the shared grip catalogue',
      exact: true,
    })
    .check()
  await page
    .getByRole('button', { name: 'Save grip product', exact: true })
    .click()
  expect(f.products.find((p) => p.brand === 'New Preview')?.active).toBe(true)
})
test('selecting an already stocked catalogue grip preserves its fitted price and stock', async ({
  page,
}) => {
  const f = await gripsAPI(page, 'staff')
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Grips', exact: true }).click()
  await page
    .getByRole('button', { name: 'Add grip from catalogue', exact: true })
    .click()
  await page.getByLabel('Search grips', { exact: true }).fill('Grip 01')
  await page.getByRole('button', { name: 'Select grip', exact: true }).click()
  await expect(
    page.getByLabel('Fitted grip price (€)', { exact: true }),
  ).toHaveValue('5.99')
  await expect(
    page.getByText('8 physical · 2 reserved · 6 free', { exact: true }),
  ).toBeVisible()
  await page.getByLabel('Fitted grip price (€)', { exact: true }).fill('7.25')
  await page
    .getByRole('button', { name: 'Save grip offering', exact: true })
    .click()
  await expect
    .poll(
      () => f.stocks.get(`${shopID}:${f.products[0].id}`)?.fitted_price_cents,
    )
    .toBe(725)
  expect(f.stocks.get(`${shopID}:${f.products[0].id}`)?.quantity).toBe(8)
})
test('an uncertain grip receipt retries once and reserved units cannot be removed', async ({
  page,
}) => {
  const f = await gripsAPI(page)
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Grips', exact: true }).click()
  await page.getByLabel('Search grips', { exact: true }).fill('Grip 01')
  await page
    .getByRole('button', { name: 'Edit grip / stock', exact: true })
    .click()
  await page
    .getByLabel('Grip units to add or remove', { exact: true })
    .fill('2')
  await page
    .getByLabel('Grip stock reason', { exact: true })
    .fill('New delivery')
  f.failNextStock()
  await page
    .getByRole('button', { name: 'Record grip stock change', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Retry same stock change', exact: true }),
  ).toBeVisible()
  await page
    .getByRole('button', { name: 'Retry same stock change', exact: true })
    .click()
  await expect(
    page.getByText('10 physical · 2 reserved · 8 free', { exact: true }),
  ).toBeVisible()
  expect(f.movements).toHaveLength(1)
  const writes = f.writes.filter((w) => w.path.endsWith('/stock-events'))
  expect(writes).toHaveLength(2)
  expect(writes[1]).toEqual(writes[0])
  await page
    .getByLabel('Grip units to add or remove', { exact: true })
    .fill('-20')
  await page
    .getByLabel('Grip stock reason', { exact: true })
    .fill('Bad correction')
  await page
    .getByRole('button', { name: 'Record grip stock change', exact: true })
    .click()
  await expect(page.getByRole('alert')).toContainText('reserved units')
  expect(f.movements).toHaveLength(1)
})
test('optional grips paginate without losing selection and the quoted fee includes fitting', async ({
  page,
}) => {
  const f = await gripsAPI(page, 'customer')
  await book(page, f.racket.id)
  await expect(
    page.getByRole('radio', { name: 'No new grip', exact: true }),
  ).toBeChecked()
  await page
    .getByRole('button', { name: 'Request server quote', exact: false })
    .click()
  expect(f.quotes[0].configuration.grip_product_id).toBeUndefined()
  await page
    .locator('.grip-picker')
    .getByRole('button', { name: /Preview Grip 01/ })
    .click()
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toHaveCount(0)
  await page
    .locator('.grip-picker')
    .getByRole('button', { name: 'Next', exact: true })
    .click()
  await expect(page.locator('.grip-picker .selected-string')).toContainText(
    'Preview Grip 01',
  )
  await page
    .getByRole('button', { name: 'Request server quote', exact: false })
    .click()
  await expect(page.locator('.booking-grip-quoted')).toContainText(
    'Preview Grip 01',
  )
  await expect(
    page
      .locator('.booking-reviewed .booking-summary-line')
      .filter({ hasText: 'Grip including fitting' }),
  ).toContainText('€5.99')
  expect(f.quotes[1].total_cents).toBe(3298)
  await page
    .getByRole('button', { name: 'Book — pay cash to stringer', exact: true })
    .click()
  await page.getByRole('link', { name: 'View order', exact: true }).click()
  await expect(
    page.getByRole('region', { name: 'Booking & payment' }),
  ).toContainText('Preview Grip 01')
})
test('zero-price grips stay selectable and sold-out saved choices require a new choice', async ({
  page,
}) => {
  const f = await gripsAPI(page, 'customer')
  await book(page, f.racket.id)
  await page
    .locator('.grip-picker')
    .getByRole('button', { name: /Preview Grip 02/ })
    .click()
  await page
    .getByRole('button', { name: 'Request server quote', exact: false })
    .click()
  await expect(
    page
      .locator('.booking-reviewed .booking-summary-line')
      .filter({ hasText: 'Grip including fitting' }),
  ).toContainText('€0.00')
  f.stocks.get(`${shopID}:${f.products[1].id}`)!.quantity = 0
  await page.reload()
  await expect(page.getByRole('alert')).toContainText(
    'selected grip is no longer available',
  )
  await expect(
    page.getByRole('button', { name: 'Request server quote', exact: false }),
  ).toBeDisabled()
  await page.getByRole('radio', { name: 'No new grip', exact: true }).check()
  await expect(
    page.getByRole('button', { name: 'Request server quote', exact: false }),
  ).toBeEnabled()
})
test('switching businesses while a grip editor loads cannot reveal the previous business stock', async ({
  page,
}) => {
  const f = await gripsAPI(page)
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Grips', exact: true }).click()
  await page.getByLabel('Search grips', { exact: true }).fill('Grip 01')
  await page.route(
    `**/stringers/${shopID}/manage/grips?grip_product_id=*`,
    async (route) => {
      await new Promise((r) => setTimeout(r, 400))
      await route.fallback()
    },
  )
  const late = page.waitForResponse((r) =>
    r.url().includes(`/stringers/${shopID}/manage/grips?grip_product_id=`),
  )
  await page
    .getByRole('button', { name: 'Edit grip / stock', exact: true })
    .click()
  await page
    .getByRole('combobox', { name: 'Your business', exact: true })
    .selectOption(otherShopID)
  await page.getByRole('button', { name: 'Grips', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: /Other business grip/ }),
  ).toBeVisible()
  await late
  await expect(
    page.getByRole('region', { name: 'Grip offering settings' }),
  ).toHaveCount(0)
  expect(f.stocks.get(`${shopID}:${f.products[0].id}`)?.quantity).toBe(8)
})
test('grip choices and stock controls fit a 320px screen', async ({ page }) => {
  const f = await gripsAPI(page)
  await page.setViewportSize({ width: 320, height: 700 })
  await book(page, f.racket.id)
  await page
    .locator('.grip-picker')
    .getByRole('button', { name: /Preview Grip 01/ })
    .click()
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
  await page.screenshot({
    path: '/tmp/restringing-grip-booking.png',
    fullPage: true,
  })
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Grips', exact: true }).click()
  await page.getByLabel('Search grips', { exact: true }).fill('Grip 01')
  await page
    .getByRole('button', { name: 'Edit grip / stock', exact: true })
    .click()
  await expect(
    page.getByRole('region', { name: 'Grip offering settings' }),
  ).toBeVisible()
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
  await page.screenshot({
    path: '/tmp/restringing-grip-stock.png',
    fullPage: true,
  })
})
