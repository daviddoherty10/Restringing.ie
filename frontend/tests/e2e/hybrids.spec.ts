import { expect, test, type Page } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
import { shopID } from './dashboard-fixture.ts'
async function book(page: Page, racket: string) {
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(racket)
  await page
    .getByRole('radio', {
      name: 'Hybrid — choose mains and crosses',
      exact: true,
    })
    .check()
}
const picker = (page: Page, component: string) =>
  page.getByRole('region', {
    name: `${component} string selection`,
    exact: true,
  })
async function select(page: Page, component: string, name: string) {
  await picker(page, component)
    .getByRole('button', { name: new RegExp(name) })
    .click()
}
test('hybrid choices survive searches, pages and reload with free mains', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await expect(
    page.getByRole('button', { name: '2 knots', exact: true }),
  ).toHaveCount(0)
  await select(page, 'mains', 'Preview String 01')
  await select(page, 'crosses', 'Preview String 02')
  await expect(
    page
      .getByRole('combobox', { name: 'Pre-stretch', exact: true })
      .locator('option'),
  ).toHaveCount(1)
  await picker(page, 'mains')
    .getByRole('button', { name: 'Next', exact: true })
    .click()
  await picker(page, 'crosses')
    .getByRole('searchbox', { name: 'Search crosses strings', exact: true })
    .fill('String 03')
  await expect(
    picker(page, 'mains').getByText(/Selected mains: Preview String 01/),
  ).toBeVisible()
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(f.quotes[0].configuration).toMatchObject({
    hybrid: true,
    variant_id: f.variants[0].id,
    crosses_variant_id: f.variants[1].id,
    knots: 4,
  })
  await expect(
    page.getByText('Mains string material (half bed)', { exact: true }),
  ).toBeVisible()
  expect(
    f.quotes[0].breakdown.find((l) => l.code === 'mains_strings')?.amount_cents,
  ).toBe(0)
  await select(page, 'crosses', 'Preview String 03')
  await expect(
    page.getByText('Mains string material (half bed)', { exact: true }),
  ).toHaveCount(0)
  await page.reload()
  await expect(
    picker(page, 'mains').getByText(/Selected mains: Preview String 01/),
  ).toBeVisible()
  await expect(
    picker(page, 'crosses').getByText(/Selected crosses: Preview String 03/),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
})
test('ambiguous hybrid purchase keeps the exact key and locks component changes', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await select(page, 'mains', 'Preview String 01')
  await select(page, 'crosses', 'Preview String 02')
  await page.getByRole('button', { name: /Request server quote/ }).click()
  f.failNextOrder()
  await page
    .getByRole('button', { name: 'Book — pay cash to stringer', exact: true })
    .click()
  await expect(page.getByText(/Connection interrupted/)).toBeVisible()
  await expect(
    picker(page, 'crosses').getByRole('button', { name: /Preview String 03/ }),
  ).toBeDisabled()
  f.sellOut(f.variants[1].id)
  await page.reload()
  await page
    .getByRole('button', { name: 'Retry pending booking', exact: true })
    .click()
  await expect(page.getByText(/Booking created/)).toBeVisible()
  expect(f.requests).toHaveLength(2)
  expect(f.requests[1]).toEqual(f.requests[0])
})
test('sold-out restored components block quotes and full-bed remains available', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await select(page, 'mains', 'Preview String 01')
  await select(page, 'crosses', 'Preview String 02')
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
  f.sellOut(f.variants[1].id)
  await page.reload()
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeDisabled()
  await expect(
    page.getByText(/A selected hybrid string is no longer available/),
  ).toBeVisible()
  await page
    .getByRole('radio', {
      name: 'Full bed — one string throughout',
      exact: true,
    })
    .check()
  await page.getByRole('button', { name: /Preview String 01/ }).click()
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(f.quotes.at(-1)?.configuration.hybrid).toBeUndefined()
  expect(f.quotes.at(-1)?.configuration.crosses_variant_id).toBeUndefined()
})
test('only one full-bed price is saved and hybrid prices are derived',async({page})=>{
 const f=await hybridsAPI(page);await page.goto('/manage');await page.getByRole('button',{name:'Inventory',exact:true}).click();await page.getByRole('button',{name:'Edit offering',exact:true}).first().click()
 await expect(page.getByLabel('Half-racket material price (€)',{exact:true})).toHaveCount(0)
 await page.getByLabel('Full-bed material price (€)',{exact:true}).fill('12.99');await page.getByRole('button',{name:'Save string offering',exact:true}).click();await expect(page.getByText(/Offering saved/)).toBeVisible();expect(f.writes.at(-1)?.body).toEqual({material_price_cents:1299,available:true,track_stock:true,track_hybrid_stock:true})
})
test('hybrid capability requires four knots and can be disabled', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Services', exact: true }).click()
  await page
    .getByRole('checkbox', { name: 'Four-knot stringing', exact: true })
    .uncheck()
  await page
    .getByRole('button', { name: 'Save service settings', exact: true })
    .click()
  await expect(
    page.getByText('Hybrid stringing requires four-knot support.', {
      exact: true,
    }),
  ).toBeVisible()
  expect(f.writes.filter((w) => w.path.includes('/services/'))).toHaveLength(0)
  await page
    .getByRole('checkbox', {
      name: 'Offer hybrid mains and crosses (half-racket stock or own strings, four knots)',
      exact: true,
    })
    .uncheck()
  await page
    .getByRole('button', { name: 'Save service settings', exact: true })
    .click()
  await expect(
    page.getByText('Service settings saved.', { exact: true }),
  ).toBeVisible()
  expect(f.service.hybrid).toBe(false)
})
test('stringers receive hybrid portions without entering any string length', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await page.goto('/manage')
  await page.getByRole('button', { name: 'Inventory', exact: true }).click()
  await page
    .getByRole('button', { name: 'Edit offering', exact: true })
    .first()
    .click()
  await page
    .getByRole('combobox', { name: 'Stock type', exact: true })
    .selectOption('half_racket')
  await page
    .getByLabel('Number of half-racket portions', { exact: true })
    .fill('24')
  await expect(
    page.getByLabel('Length per set (metres)', { exact: true }),
  ).toHaveCount(0)
  await expect(
    page.getByLabel('Reel length (metres)', { exact: true }),
  ).toHaveCount(0)
  await page.getByRole('button', { name: 'Receive stock', exact: true }).click()
  await expect(
    page.getByText('Stock received and recorded.', { exact: true }),
  ).toBeVisible()
  expect(f.writes.at(-1)?.body).toEqual({
    kind: 'half_racket',
    quantity: 24,
    set_length_mm: 0,
  })
})
test('hybrid controls fit a narrow phone', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 })
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await select(page, 'mains', 'Preview String 01')
  await select(page, 'crosses', 'Preview String 02')
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
  await page.screenshot({
    path: `test-results/hybrid-booking-${test.info().project.name}.png`,
    fullPage: true,
  })
})

test('slow component restoration preserves the saved draft and prevents edits until complete', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await select(page, 'mains', 'Preview String 01')
  await select(page, 'crosses', 'Preview String 02')
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
  let release: () => void = () => {}
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/hybrid-inventory?**', async (route) => {
    if (new URL(route.request().url()).searchParams.has('variant_id'))
      await gate
    await route.fallback()
  })
  await page.reload()
  await expect(
    page.getByText('Restoring your saved setup…', { exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('radio', {
      name: 'Full bed — one string throughout',
      exact: true,
    }),
  ).toBeDisabled()
  const draft = await page.evaluate(
    (id) => JSON.parse(sessionStorage.getItem('restringing:booking:' + id)!),
    shopID,
  )
  expect(draft.variantID).toBe(f.variants[0].id)
  expect(draft.crossesVariantID).toBe(f.variants[1].id)
  release()
  await expect(
    page.getByText('Restoring your saved setup…', { exact: true }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
})

test('customer supplied hybrids select global catalogue strings without material charges', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await page
    .getByRole('checkbox', { name: "I'll provide my own strings", exact: true })
    .check()
  await picker(page, 'mains')
    .getByRole('searchbox', { name: 'Search mains strings', exact: true })
    .fill('Customer Only')
  await select(page, 'mains', 'Preview Customer Only String')
  await select(page, 'crosses', 'Preview String 03')
  await expect(picker(page, 'mains').getByText(/half racket/)).toHaveCount(0)
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(f.quotes.at(-1)?.configuration).toMatchObject({
    string_source: 'customer',
    hybrid: true,
    variant_id: f.customerOnly.id,
    crosses_variant_id: f.variants[2].id,
  })
  expect(f.quotes.at(-1)?.breakdown.map((l) => l.code)).toEqual([
    'labour',
    'knots',
    'own_strings',
  ])
  await page.reload()
  await expect(
    picker(page, 'mains').getByText(
      /Selected mains: Preview Customer Only String/,
    ),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
})
test('customers describe missing own mains and crosses independently and restore the draft', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await page
    .getByRole('checkbox', { name: "I'll provide my own strings", exact: true })
    .check()
  for (const component of ['mains', 'crosses'])
    await picker(page, component)
      .getByRole('checkbox', {
        name: `My ${component} string is not in the catalogue`,
        exact: true,
      })
      .check()
  await page
    .getByLabel('Your mains string details', { exact: true })
    .fill('My gut 1.30')
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeDisabled()
  await page
    .getByLabel('Your crosses string details', { exact: true })
    .fill('My poly 1.25')
  await expect(
    page
      .getByRole('combobox', { name: 'Pre-stretch', exact: true })
      .locator('option'),
  ).toHaveCount(1)
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(f.quotes.at(-1)?.configuration).toMatchObject({
    string_source: 'customer',
    hybrid: true,
    customer_mains_description: 'My gut 1.30',
    customer_crosses_description: 'My poly 1.25',
  })
  expect(f.quotes.at(-1)?.configuration.variant_id).toBeUndefined()
  await page.reload()
  await expect(
    page.getByLabel('Your mains string details', { exact: true }),
  ).toHaveValue('My gut 1.30')
  await expect(
    page.getByLabel('Your crosses string details', { exact: true }),
  ).toHaveValue('My poly 1.25')
  await expect(
    page.getByRole('button', { name: /Request server quote/ }),
  ).toBeEnabled()
})
test('derived hybrid material totals stay the same when mains and crosses are swapped', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await book(page, f.racket.id)
  await select(page, 'mains', 'Preview String 02')
  await select(page, 'crosses', 'Preview String 03')
  await page.getByRole('button', { name: /Request server quote/ }).click()
  const first = f.quotes[0]
  await select(page, 'mains', 'Preview String 03')
  await select(page, 'crosses', 'Preview String 02')
  await page.getByRole('button', { name: /Request server quote/ }).click()
  const second = f.quotes[1]
  expect(first.total_cents).toBe(second.total_cents)
  expect(
    first.breakdown.find((l) => l.code === 'mains_strings')?.amount_cents,
  ).toBe(
    second.breakdown.find((l) => l.code === 'crosses_strings')?.amount_cents,
  )
})
