import { expect, test, type Page } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
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
test('both supported knot choices are visible and two knots can be quoted', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await setup(page, f.racket.id)
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
test('a four-knot-only racket explains why the stringer’s two-knot option is unavailable', async ({
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
  await setup(page, f.racket.id)
  await expect(
    page.getByText(/this racket’s verified pattern allows four knots only/),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: '2 knots', exact: true }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('button', { name: '4 knots', exact: true }),
  ).toBeVisible()
})
test('lb input shows applied settings, quotes canonical kg, and survives reload and unit switching', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await setup(page, f.racket.id)
  await page
    .getByRole('combobox', { name: 'Tension unit', exact: true })
    .selectOption('lb')
  await page
    .getByRole('textbox', { name: 'Mains tension (lb)', exact: true })
    .fill('48')
  await expect(
    page.getByText('Applied: 22 kg (48.50 lb).', { exact: false }),
  ).toBeVisible()
  await page
    .getByRole('checkbox', {
      name: 'Use separate crosses tension',
      exact: true,
    })
    .check()
  await page
    .getByRole('textbox', { name: 'Crosses tension (lb)', exact: true })
    .fill('53')
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({
    mains_tension_centi_kg: 2200,
    crosses_tension_centi_kg: 2400,
  })
  await page
    .getByRole('combobox', { name: 'Tension unit', exact: true })
    .selectOption('kg')
  await expect(
    page.getByRole('textbox', { name: 'Mains tension (kg)', exact: true }),
  ).toHaveValue('22')
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toBeVisible()
  await page
    .getByRole('combobox', { name: 'Tension unit', exact: true })
    .selectOption('lb')
  await page.reload()
  await expect(
    page.getByRole('combobox', { name: 'Tension unit', exact: true }),
  ).toHaveValue('lb')
  await expect(
    page.getByRole('textbox', { name: 'Mains tension (lb)', exact: true }),
  ).toHaveValue('48.50')
  await expect(review(page)).toBeEnabled()
  await page
    .getByRole('textbox', { name: 'Mains tension (lb)', exact: true })
    .fill('200')
  await expect(review(page)).toBeDisabled()
  await page.setViewportSize({ width: 320, height: 740 })
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
})
