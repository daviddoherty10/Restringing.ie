import { expect, test, type Page } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
import { shopID } from './dashboard-fixture.ts'
async function setup(page: Page) {
  await page.goto(`/stringers/${shopID}/book`)
  await page.getByRole('checkbox', {name: 'I know my tension', exact: true}).check()
  await page.getByRole('button', { name: /Preview String 01/ }).click()
}
const review = (page: Page) =>
  page.getByRole('button', { name: /Request server quote/ })
test('both supported knot choices are visible and two knots can be quoted', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await setup(page)
  for (const n of [2, 4])
    await expect(
      page.getByRole('button', { name: `${n} knots`, exact: true }),
    ).toBeVisible()
  await page.getByRole('button', { name: '2 knots', exact: true }).click()
  await expect(
    page.getByRole('button', { name: '2 knots', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true')
  await review(page).click()
  expect(f.quotes[0].configuration.knots).toBe(2)
  await page
    .getByRole('radio', {
      name: 'Hybrid — choose mains and crosses',
      exact: true,
    })
    .check()
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
    page.getByRole('button', { name: '2 knots', exact: true }),
  ).toBeVisible()
})
test('a four-knot-only service does not offer two knots', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await page.route('**/booking-configuration?*', (route) =>
    route.fulfill({
      json: {
        stringer_id: shopID,
        racket: { ...f.racket, allowed_knots: [4] },
        service: f.service,
        min_tension_centi_kg: 1800,
        max_tension_centi_kg: 3000,
        step_centi_kg: 50,
        max_difference_centi_kg: 200,
        allowed_knots: [4],
        notice: 'Verified four-knot-only fixture.',
      },
    }),
  )
  await setup(page)
  await expect(
    page.getByText(/Four-knot stringing is the available method/),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: '2 knots', exact: true }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('button', { name: '4 knots', exact: true }),
  ).toBeVisible()
})
test('pound tensions are quoted exactly, survive reload, and clear when units change', async ({ page }) => {
  const f = await hybridsAPI(page)
  await setup(page)
  const unit = page.getByRole('combobox', { name: 'Tension unit', exact: true })
  const mains = page.getByRole('textbox', { name: 'Mains tension (lb)', exact: true })
  await unit.selectOption('lb')
  await mains.fill('48.33')
  await page.getByRole('checkbox', { name: 'Use separate crosses tension', exact: true }).check()
  await page.getByRole('textbox', { name: 'Crosses tension (lb)', exact: true }).fill('47')
  await expect(page.getByText('48.33 / 47 lb', { exact: true })).toBeVisible()
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({
    mains_tension_centi_kg: 0,
    crosses_tension_centi_kg: 0,
    mains_tension_centi_lb: 4833,
    crosses_tension_centi_lb: 4700,
  })
  await page.reload()
  await expect(unit).toHaveValue('lb')
  await expect(mains).toHaveValue('48.33')
  await expect(review(page)).toBeEnabled()
  await mains.fill('200')
  await expect(review(page)).toBeDisabled()
  await unit.selectOption('kg')
  await expect(page.getByRole('textbox', { name: 'Mains tension (kg)', exact: true })).toHaveValue('')
  await expect(review(page)).toBeDisabled()
  await page.setViewportSize({ width: 320, height: 740 })
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('choosing different saved rackets restores their own string and tension', async ({ page }) => {
  const f = await hybridsAPI(page)
  const base = {
    sport: 'tennis', string_source: 'stock', racket_snapshot: 'Tennis racket', string_snapshot: '',
    mains_tension_centi_kg: 0, crosses_tension_centi_kg: 0,
    knots: 4, stencil: false, instructions: '',
  }
  await page.route('**/orders/saved-rackets', route => route.fulfill({ json: [
    { order_id: 'adult', configuration: { ...base, racket_details: 'My Blade', variant_id: f.variants[0].id, mains_tension_centi_lb: 4900, crosses_tension_centi_lb: 4800 } },
    { order_id: 'junior', configuration: { ...base, racket_details: 'Family junior racket', variant_id: f.variants[1].id, tension_pending: true } },
  ] }))
  await page.goto(`/stringers/${shopID}/book`)
  const saved = page.getByRole('combobox', { name: 'Use a saved racket', exact: true })
  await saved.selectOption('adult')
  await expect(page.getByLabel('Racket details (optional)')).toHaveValue('My Blade')
  await expect(page.getByText('49 / 48 lb', { exact: true })).toBeVisible()
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({ racket_details: 'My Blade', variant_id: f.variants[0].id, mains_tension_centi_lb: 4900 })
  await saved.selectOption('junior')
  await expect(page.getByLabel('Racket details (optional)')).toHaveValue('Family junior racket')
  await expect(page.getByLabel('Tension guide')).toHaveValue('estimate')
  await review(page).click()
  expect(f.quotes[1].configuration).toMatchObject({ racket_details: 'Family junior racket', variant_id: f.variants[1].id, tension_pending: true })
  expect(f.quotes[1].configuration.mains_tension_centi_lb).toBeUndefined()
})
