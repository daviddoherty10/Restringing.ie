import { expect, test, type Page } from '@playwright/test'
import { dashboardAPI } from './dashboard-fixture.ts'
import { hybridsAPI } from './hybrids-fixture.ts'
import { shopID } from './dashboard-fixture.ts'
async function newOffering(page: Page) {
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Inventory', exact: true }).click()
  await page
    .getByRole('button', { name: 'Add from catalogue', exact: true })
    .click()
  await page
    .getByRole('searchbox', { name: 'Search catalogue', exact: true })
    .fill('New Catalogue String')
  await page.getByRole('button', { name: 'Select string', exact: true }).click()
}
test('a new offering only needs a full-bed price and availability', async ({
  page,
}) => {
  const f = await dashboardAPI(page)
  await newOffering(page)
  await expect(
    page.getByLabel('Full-bed material price (€)', { exact: true }),
  ).toHaveValue('')
  await expect(
    page.getByRole('checkbox', {
      name: 'In stock — available for booking',
      exact: true,
    }),
  ).toBeChecked()
  await expect(
    page.getByRole('button', { name: 'Receive stock', exact: true }),
  ).toHaveCount(0)
  await expect(
    page.getByLabel('Half-racket material price (€)', { exact: true }),
  ).toHaveCount(0)
  await page
    .getByLabel('Full-bed material price (€)', { exact: true })
    .fill('12.99')
  await page
    .getByRole('button', { name: 'Save string offering', exact: true })
    .click()
  await expect(
    page.getByText(
      'Offering saved. Customers can book while it is marked in stock.',
      { exact: true },
    ),
  ).toBeVisible()
  expect(f.writes.at(-1)?.body).toEqual({
    material_price_cents: 1299,
    available: true,
    track_stock: false,
  })
  expect(f.writes.filter((w) => w.method === 'POST')).toHaveLength(0)
  await expect(
    page.getByRole('button', { name: 'Receive stock', exact: true }),
  ).toHaveCount(0)
  await page
    .getByRole('button', { name: 'Close string settings', exact: true })
    .click()
  await page.getByRole('button', { name: 'Offerings', exact: true }).click()
  await page
    .getByRole('searchbox', { name: 'Search offerings', exact: true })
    .fill('New Catalogue String')
  await page.getByRole('button', { name: 'Edit offering', exact: true }).click()
  await expect(
    page.getByLabel('Full-bed material price (€)', { exact: true }),
  ).toHaveValue('12.99')
})
test('quantity tracking is an optional setting that reveals stock receipts after saving', async ({
  page,
}) => {
  const f = await dashboardAPI(page)
  await newOffering(page)
  await page
    .getByLabel('Full-bed material price (€)', { exact: true })
    .fill('14')
  await page
    .getByText('Advanced stock settings (optional)', { exact: true })
    .click()
  await page
    .getByRole('checkbox', { name: 'Track stock quantities', exact: true })
    .check()
  await page
    .getByRole('button', { name: 'Save string offering', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Receive stock', exact: true }),
  ).toBeVisible()
  expect(f.writes.at(-1)?.body).toMatchObject({
    track_stock: true,
    material_price_cents: 1400,
  })
  await page
    .getByRole('combobox', { name: 'Stock type', exact: true })
    .selectOption('half_racket')
  await page
    .getByLabel('Number of half-racket portions', { exact: true })
    .fill('8')
  await page.getByRole('button', { name: 'Receive stock', exact: true }).click()
  await expect(
    page.getByText('Stock received and recorded.', { exact: true }),
  ).toBeVisible()
  expect(f.writes.at(-1)?.body).toEqual({
    kind: 'half_racket',
    quantity: 8,
    set_length_mm: 0,
  })
})
test('odd-cent hybrid prices round the combined material once and preserve same-string totals', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  f.variants[0].material_price_cents = 1299
  f.variants[1].material_price_cents = 1000
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(f.racket.id)
  await page
    .getByRole('radio', {
      name: 'Hybrid — choose mains and crosses',
      exact: true,
    })
    .check()
  const mains = page.getByRole('region', {
      name: 'mains string selection',
      exact: true,
    }),
    cross = page.getByRole('region', {
      name: 'crosses string selection',
      exact: true,
    })
  await mains.getByRole('button', { name: /Preview String 01/ }).click()
  await cross.getByRole('button', { name: /Preview String 02/ }).click()
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(
    f.quotes[0].breakdown
      .filter((l) => l.code.endsWith('_strings'))
      .reduce((sum, l) => sum + l.amount_cents, 0),
  ).toBe(1150)
  await cross.getByRole('button', { name: /Preview String 01/ }).click()
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(
    f.quotes[1].breakdown
      .filter((l) => l.code.endsWith('_strings'))
      .reduce((sum, l) => sum + l.amount_cents, 0),
  ).toBe(1299)
})
test('simple offering form fits a narrow phone', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 })
  await dashboardAPI(page)
  await newOffering(page)
  await page
    .getByLabel('Full-bed material price (€)', { exact: true })
    .fill('14.99')
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
  await page.screenshot({
    path: `test-results/simple-offering-${test.info().project.name}.png`,
    fullPage: true,
  })
})
