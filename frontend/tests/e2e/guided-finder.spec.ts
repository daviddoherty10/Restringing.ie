import { expect, test, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { guidedFinderAPI } from './guided-finder-fixture.ts'
import { shopID } from './dashboard-fixture.ts'
async function openFinder(page: Page, chooseSport = false) {
  await page.goto(`/stringers/${shopID}/book`)
  if (chooseSport) await page.getByRole('combobox', { name: 'Sport', exact: true }).selectOption('tennis')
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
  await openFinder(page)
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
    mains_tension_centi_kg: 0,
    crosses_tension_centi_kg: 0,
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
      name: 'Confirm booking',
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
  await openFinder(page)
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
    .getByRole('radio', { name: /^Spin / })
    .check()
  await find(page).click()
  await expect(page.getByRole('article')).toContainText(
    'Solinco Hyper-G + HEAD Velocity MLT',
  )
  await expect(page.locator('.setup-sketch-hybrid')).toBeVisible()
  await expect(page.getByText('Two strings, working together', { exact: true })).toBeVisible()
  await expect(page.locator('.setup-key-cross')).toContainText('Crosses (horizontal): HEAD Velocity MLT')
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
  await openFinder(page)
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
  await page.route('**/services', route => route.fulfill({ json: [
    f.service,
    { ...f.service, sport: 'squash' },
    { ...f.service, sport: 'badminton', active: false },
  ] }))
  await openFinder(page, true)
  await expect(page.getByRole('option', { name: 'Squash' })).toHaveCount(1)
  await expect(page.getByRole('option', { name: 'Badminton' })).toHaveCount(0)
  f.delay(350)
  await find(page).click()
  await expect.poll(() => f.answers.length).toBe(1)
  await page
    .getByRole('radio', { name: /^Easy depth / })
    .check()
  await page.waitForTimeout(450)
  await expect(choose(page)).toHaveCount(0)
  await find(page).click()
  await expect(choose(page)).toBeVisible()
  await page
    .getByRole('combobox', { name: 'Sport', exact: true })
    .selectOption('')
  await expect(choose(page)).toHaveCount(0)
})

test('illustrated goals support keyboard choices, reduced motion and readable mobile results', async ({ page }) => {
  const f = await guidedFinderAPI(page)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await openFinder(page)
  const options = page.getByRole('group', { name: 'What would help you most?' })
  await expect(options.getByRole('radio')).toHaveCount(6)
  const balanced = options.getByRole('radio', { name: /^All-round / })
  await expect(balanced).toBeChecked()
  await balanced.focus()
  await page.keyboard.press('ArrowRight')
  await expect(options.getByRole('radio', { name: /^Comfort / })).toBeChecked()
  expect(await page.locator('.goal-selected path').first().evaluate(el => getComputedStyle(el).animationName)).toBe('none')
  await find(page).click()
  expect(f.answers[0].preferences?.comfort).toBe(5)
  await expect(page.getByText('One string, in both directions', { exact: true })).toBeVisible()
  await expect(page.getByText('Crosses (horizontal): the same string', { exact: true })).toBeVisible()
  const audit = await new AxeBuilder({ page }).include('.guided-finder').withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze()
  expect(audit.violations.map(v => ({ id: v.id, nodes: v.nodes.map(n => n.failureSummary) }))).toEqual([])
  await page.locator('.goal-grid').screenshot({ path: `/tmp/restringing-visual-finder-${test.info().project.name}.png` })
})

test('the 26-inch junior choice is sent to the picker and can be cleared', async ({ page }) => {
  const f = await guidedFinderAPI(page)
  await openFinder(page)
  const junior = page.getByRole('checkbox', { name: 'This is a 26-inch junior racket' })
  await junior.check()
  await find(page).click()
  await expect(choose(page)).toBeVisible()
  expect(f.answers[0].junior_26).toBe(true)
  await junior.uncheck()
  await expect(choose(page)).toHaveCount(0)
  await find(page).click()
  await expect(choose(page)).toBeVisible()
  expect(f.answers[1].junior_26).toBeUndefined()
})

test('single-sport bookings skip sport selection and preserve an undecided tension', async ({ page }) => {
  const f = await guidedFinderAPI(page)
  const catalogueReads: string[] = []
  page.on('request', req => { if (new URL(req.url()).pathname.endsWith('/rackets')) catalogueReads.push(req.url()) })
  await page.goto(`/stringers/${shopID}/book`)
  await expect(page.getByRole('combobox', { name: 'Racket model', exact: true })).toHaveCount(0)
  await page.getByLabel('Racket details (optional)').fill('An older racket I cannot find online')
  await expect(page.getByRole('radio', { name: 'Help me decide', exact: true })).toBeChecked()
  await expect(page.getByRole('combobox', { name: 'Sport', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: /Help me choose a complete setup/ }).click()
  await find(page).click()
  expect(f.answers[0].sport).toBe('tennis')
  expect(f.answers[0].racket_model_id).toBeUndefined()
  await choose(page).click()
  await review(page).click()
  expect(f.quotes[0].configuration).toMatchObject({ sport: 'tennis', racket_details: 'An older racket I cannot find online', tension_pending: true, mains_tension_centi_kg: 0, crosses_tension_centi_kg: 0 })
  expect(f.quotes[0].configuration.racket_model_id).toBeUndefined()
  await page.reload()
  await expect(page.getByLabel('Racket details (optional)')).toHaveValue('An older racket I cannot find online')
  await expect(page.getByRole('radio', { name: 'Help me decide', exact: true })).toBeChecked()
  await expect(page.getByRole('combobox', { name: 'Sport', exact: true })).toHaveCount(0)
  await page.getByRole('radio', { name: 'I know my tension', exact: true }).check()
  await page.getByLabel('Mains tension (kg)', { exact: true }).fill('24')
  await review(page).click()
  expect(f.quotes.at(-1)?.configuration).toMatchObject({ tension_pending: false, mains_tension_centi_kg: 2400, crosses_tension_centi_kg: 2400 })
  expect(catalogueReads).toEqual([])
  await page.setViewportSize({ width: 320, height: 740 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  const audit = await new AxeBuilder({ page }).include('.booking-form').withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze()
  expect(audit.violations.map(v => ({ id: v.id, nodes: v.nodes.map(n => n.failureSummary) }))).toEqual([])
})
