import { expect, test, type Page } from '@playwright/test'
import { guidedFinderAPI } from './guided-finder-fixture.ts'
import { shopID } from './dashboard-fixture.ts'
async function openFinder(page: Page, racket: string) {
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(racket)
  await page
    .getByRole('button', { name: /Help me choose a complete setup/ })
    .click()
}
const find = (page: Page) =>
  page.getByRole('button', { name: 'Find my complete setup', exact: true })
const choose = (page: Page) =>
  page.getByRole('button', { name: 'Use this setup', exact: true })
const review = (page: Page) =>
  page.getByRole('button', { name: /Request server quote/ })
test('a beginner gets a complete explained setup without choosing technical options', async ({
  page,
}) => {
  const f = await guidedFinderAPI(page)
  await openFinder(page, f.racket.id)
  await expect(
    page.getByRole('combobox', {
      name: 'How would you describe your play?',
      exact: true,
    }),
  ).toHaveValue('new')
  await find(page).click()
  const suggestion = page.getByRole('article', {
    name: 'Suggested setup 1',
    exact: true,
  })
  await expect(suggestion).toContainText('HEAD Velocity MLT')
  await expect(suggestion).toContainText('€26.00 estimated, including labour')
  await expect(suggestion.getByText(/weight 4/)).not.toBeVisible()
  await choose(page).click()
  await expect(
    page.getByText('Your setup is ready.', { exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).not.toBeVisible()
  await expect(
    page.getByRole('combobox', { name: 'Pre-stretch', exact: true }),
  ).not.toBeVisible()
  await expect(review(page)).toBeEnabled()
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({
    variant_id: f.variants[0].id,
    knots: 4,
    mains_tension_centi_kg: 2400,
    crosses_tension_centi_kg: 2400,
    prestretch_percent: 0,
    stencil: false,
    express: false,
    string_source: 'stock',
  })
  expect(f.quotes[0].configuration.hybrid).toBeUndefined()
  await page.reload()
  await expect(
    page.getByText('Your setup is ready.', { exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).not.toBeVisible()
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toHaveCount(0)
  await expect(review(page)).toBeEnabled()
  await page
    .getByText('Adjust stringing settings (optional)', { exact: true })
    .click()
  await expect(
    page.getByRole('group', { name: 'Knots', exact: true }),
  ).toBeVisible()
})
test('a regular spin player can apply both hybrid components and a softer-feel answer changes the suggestion', async ({
  page,
}) => {
  const f = await guidedFinderAPI(page)
  await openFinder(page, f.racket.id)
  await page
    .getByRole('combobox', {
      name: 'How would you describe your play?',
      exact: true,
    })
    .selectOption('club')
  await page
    .getByRole('combobox', {
      name: 'How often do you usually play?',
      exact: true,
    })
    .selectOption('2')
  await page
    .getByRole('combobox', { name: 'What would help you most?', exact: true })
    .selectOption('spin')
  await find(page).click()
  await expect(page.getByRole('article')).toContainText(
    'Solinco Hyper-G + HEAD Velocity MLT',
  )
  await page
    .getByRole('checkbox', { name: /Please put a softer feel first/ })
    .check()
  await expect(page.getByRole('article')).toHaveCount(0)
  await find(page).click()
  await expect(page.getByRole('article')).not.toContainText('Solinco')
  await page
    .getByRole('checkbox', { name: /Please put a softer feel first/ })
    .uncheck()
  await find(page).click()
  await choose(page).click()
  await expect(review(page)).toBeEnabled()
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({
    hybrid: true,
    variant_id: f.variants[1].id,
    crosses_variant_id: f.variants[0].id,
    knots: 4,
    prestretch_percent: 0,
  })
})
test('empty matches and finder errors are actionable on a narrow phone', async ({
  page,
}) => {
  const f = await guidedFinderAPI(page)
  await page.setViewportSize({ width: 320, height: 740 })
  await openFinder(page, f.racket.id)
  f.fail()
  await find(page).click()
  await expect(page.getByRole('alert')).toContainText(
    'Finder temporarily unavailable.',
  )
  await page.getByRole('button', { name: 'Retry finder', exact: true }).click()
  await expect(choose(page)).toBeVisible()
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
  await page.screenshot({
    path: `test-results/guided-finder-${test.info().project.name}.png`,
    fullPage: true,
  })
  f.setEmpty(true)
  await find(page).click()
  await expect(
    page.getByRole('status').filter({ hasText: 'No complete starting setup' }),
  ).toBeVisible()
  await expect(choose(page)).toHaveCount(0)
})
test('late answers cannot replace recommendations for changed preferences or another racket', async ({
  page,
}) => {
  const f = await guidedFinderAPI(page)
  await openFinder(page, f.racket.id)
  f.delay(350)
  await find(page).click()
  await expect.poll(() => f.answers.length).toBe(1)
  await page
    .getByRole('combobox', { name: 'What would help you most?', exact: true })
    .selectOption('power')
  await page.waitForTimeout(450)
  await expect(choose(page)).toHaveCount(0)
  await find(page).click()
  await expect(choose(page)).toBeVisible()
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption('')
  await expect(choose(page)).toHaveCount(0)
})
