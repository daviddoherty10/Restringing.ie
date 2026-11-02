import { expect, test, type Page } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
import { gripsAPI } from './grips-fixture.ts'
import { shopID } from './dashboard-fixture.ts'
async function setup(page: Page, racket: string) {
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(racket)
  await page.getByRole('button', { name: /Preview String 01/ }).click()
}
const review = (page: Page) =>
  page.getByRole('button', { name: /Request server quote/ })
const hybrid = (page: Page) =>
  page.getByRole('radio', {
    name: 'Hybrid — choose mains and crosses',
    exact: true,
  })
test('booking only shows offered services and compatible pre-stretch choices', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  Object.assign(f.service, {
    hybrid: false,
    own_strings: false,
    stencil: false,
    express: false,
    different_tensions: false,
    prestretch_options: {},
  })
  await setup(page, f.racket.id)
  await expect(page.getByRole('checkbox', { name: /Add stencil/ })).toHaveCount(
    0,
  )
  await expect(
    page.getByRole('checkbox', { name: /provide my own strings/ }),
  ).toHaveCount(0)
  await expect(hybrid(page)).toHaveCount(0)
  await expect(page.locator('.booking-turnaround')).toHaveCount(0)
  await expect(page.locator('.grip-picker')).toHaveCount(0)
  await expect(
    page.getByRole('combobox', { name: 'Pre-stretch', exact: true }),
  ).toHaveCount(0)
  await expect(
    page
      .locator('.booking-summary-line')
      .filter({ hasText: /^(Grip|Turnaround)/ }),
  ).toHaveCount(0)
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({
    express: false,
    stencil: false,
    prestretch_percent: 0,
    string_source: 'stock',
  })
  expect(f.quotes[0].configuration.grip_product_id).toBeUndefined()
  Object.assign(f.service, {
    stencil: true,
    express: true,
    express_fee_cents: 0,
    express_turnaround: 'Within a day',
    prestretch_options: { '5': 0 },
  })
  await page.reload()
  await expect(
    page.getByRole('checkbox', { name: /Add stencil/ }),
  ).toBeVisible()
  await expect(
    page.getByRole('radio', { name: /Express turnaround/ }),
  ).toBeVisible()
  await expect(
    page.getByRole('combobox', { name: 'Pre-stretch', exact: true }),
  ).toBeVisible()
})
test('hybrid hides knot controls and shows material prices before requesting a quote', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  f.variants[0].material_price_cents = 1200
  f.variants[1].material_price_cents = 1800
  await setup(page, f.racket.id)
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).toBeVisible()
  await hybrid(page).check()
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).toHaveCount(0)
  const mains = page.getByRole('region', {
    name: 'mains string selection',
    exact: true,
  })
  const crosses = page.getByRole('region', {
    name: 'crosses string selection',
    exact: true,
  })
  await expect(
    mains.getByRole('button', { name: /Preview String 01/ }),
  ).toContainText('€6.00 / half bed')
  await expect(
    crosses.getByRole('button', { name: /Preview String 02/ }),
  ).toContainText('€9.00 / half bed')
  await mains.getByRole('button', { name: /Preview String 01/ }).click()
  await crosses.getByRole('button', { name: /Preview String 02/ }).click()
  expect(f.quotes).toHaveLength(0)
  await review(page).click()
  expect(f.quotes[0].configuration.knots).toBe(4)
  await page.reload()
  await expect(hybrid(page)).toBeChecked()
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).toHaveCount(0)
  await page
    .getByRole('radio', {
      name: 'Full bed — one string throughout',
      exact: true,
    })
    .check()
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).toBeVisible()
  await page.setViewportSize({ width: 320, height: 740 })
  await hybrid(page).check()
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
  await page.screenshot({
    path: `test-results/booking-options-${test.info().project.name}.png`,
    fullPage: true,
  })
})
test('withdrawn saved services are removed with a notice instead of blocking hidden controls', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  Object.assign(f.service, {
    express: true,
    express_turnaround: 'Within a day',
  })
  await setup(page, f.racket.id)
  await page.getByRole('checkbox', { name: /Add stencil/ }).check()
  await page.getByRole('radio', { name: /Express turnaround/ }).check()
  await page
    .getByRole('combobox', { name: 'Pre-stretch', exact: true })
    .selectOption('5')
  await review(page).click()
  Object.assign(f.service, {
    express: false,
    stencil: false,
    prestretch_options: {},
  })
  await page.reload()
  await expect(
    page
      .getByRole('status')
      .filter({ hasText: 'Some saved options are no longer available' }),
  ).toContainText('Some saved options are no longer available')
  await expect(page.getByRole('checkbox', { name: /Add stencil/ })).toHaveCount(
    0,
  )
  await expect(page.locator('.booking-turnaround')).toHaveCount(0)
  await expect(
    page.getByRole('combobox', { name: 'Pre-stretch', exact: true }),
  ).toHaveCount(0)
  await expect(review(page)).toBeEnabled()
  await review(page).click()
  expect(f.quotes.at(-1)?.configuration).toMatchObject({
    express: false,
    stencil: false,
    prestretch_percent: 0,
  })
})
test('grips stay searchable after no matches and an empty offering hides the add-on', async ({
  page,
}) => {
  const f = await gripsAPI(page, 'customer')
  await setup(page, f.racket.id)
  await page
    .getByRole('searchbox', { name: 'Search offered grips', exact: true })
    .fill('No match here')
  await expect(
    page.getByText(/No available grips match these filters/),
  ).toBeVisible()
  await expect(
    page.getByRole('heading', { name: 'Fresh grip?', exact: true }),
  ).toBeVisible()
  await page
    .getByRole('searchbox', { name: 'Search offered grips', exact: true })
    .fill('')
  await page
    .locator('.grip-picker')
    .getByRole('button', { name: /Preview Grip 01/ })
    .click()
  for (const g of f.stocks.values()) g.quantity = g.reserved
  await page.reload()
  await expect(
    page.getByRole('button', { name: 'Remove unavailable grip', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('heading', { name: 'Fresh grip?', exact: true }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('searchbox', { name: 'Search offered grips', exact: true }),
  ).toHaveCount(0)
  await expect(review(page)).toBeDisabled()
  await page
    .getByRole('button', { name: 'Remove unavailable grip', exact: true })
    .click()
  await expect(review(page)).toBeEnabled()
  await review(page).click()
  expect(f.quotes.at(-1)?.configuration.grip_product_id).toBeUndefined()
})
test('withdrawn own-string service clears saved supplied selections before a stock quote', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await setup(page, f.racket.id)
  await page.getByRole('checkbox', { name: /provide my own strings/ }).check()
  await hybrid(page).check()
  for (const component of ['mains', 'crosses'])
    await page
      .getByRole('region', {
        name: `${component} string selection`,
        exact: true,
      })
      .getByRole('button', { name: /Preview String 01/ })
      .click()
  await review(page).click()
  expect(f.quotes[0].configuration.string_source).toBe('customer')
  f.service.own_strings = false
  await page.reload()
  await expect(
    page
      .getByRole('status')
      .filter({ hasText: 'Some saved options are no longer available' }),
  ).toBeVisible()
  await expect(
    page.getByRole('checkbox', { name: /provide my own strings/ }),
  ).toHaveCount(0)
  await expect(review(page)).toBeDisabled()
  for (const component of ['mains', 'crosses'])
    await page
      .getByRole('region', {
        name: `${component} string selection`,
        exact: true,
      })
      .getByRole('button', { name: /Preview String 01/ })
      .click()
  await review(page).click()
  expect(f.quotes.at(-1)?.configuration.string_source).toBe('stock')
})
