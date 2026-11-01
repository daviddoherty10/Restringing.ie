import { expect, test } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
import { shopID } from './dashboard-fixture.ts'

test('legacy full-bed offerings remain selectable for either hybrid half without portion setup', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  // Match the user's older database: full-bed stock is tracked on every offering,
  // and only the first offering has explicitly configured hybrid portions.
  f.variants.forEach((v, i) => {
    v.track_stock = true
    v.track_hybrid_stock = i === 0
  })
  await page.route('**/hybrid-inventory?**', async (route) => {
    const url = new URL(route.request().url()),
      query = (url.searchParams.get('q') ?? '').toLowerCase(),
      id = url.searchParams.get('variant_id')
    const data = f.variants.filter(
      (v) =>
        v.available &&
        (!v.track_hybrid_stock || v.id === f.variants[0].id) &&
        (!id || v.id === id) &&
        (!query || `${v.brand} ${v.model}`.toLowerCase().includes(query)),
    )
    const size = Number(url.searchParams.get('page_size') || 5),
      p = Number(url.searchParams.get('page') || 1)
    await route.fulfill({
      json: {
        data: data.slice((p - 1) * size, p * size),
        total: data.length,
        page: p,
        page_size: size,
      },
    })
  })
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(f.racket.id)
  await expect(
    page.getByRole('button', { name: /Preview String/ }).first(),
  ).toBeVisible()
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
  await expect(
    mains.getByText('Page 1 of 5 · 24 results', { exact: true }),
  ).toBeVisible()
  await expect(
    cross.getByText('Page 1 of 5 · 24 results', { exact: true }),
  ).toBeVisible()
  await mains.getByRole('button', { name: 'Next', exact: true }).click()
  await mains.getByRole('button', { name: /Preview String 06/ }).click()
  await cross
    .getByRole('searchbox', { name: 'Search crosses strings', exact: true })
    .fill('String 18')
  await cross.getByRole('button', { name: /Preview String 18/ }).click()
  await page.getByRole('button', { name: /Request server quote/ }).click()
  expect(f.quotes.at(-1)?.configuration).toMatchObject({
    hybrid: true,
    variant_id: f.variants[5].id,
    crosses_variant_id: f.variants[17].id,
  })
  await expect(
    page.getByRole('button', {
      name: 'Book — pay cash to stringer',
      exact: true,
    }),
  ).toBeVisible()
})
