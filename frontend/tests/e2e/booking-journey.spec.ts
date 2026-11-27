import { expect, test } from '@playwright/test'
import { shopID } from './dashboard-fixture.ts'
import { hybridsAPI } from './hybrids-fixture.ts'

test('phone booking leads with setup choices and recovers from an empty string search', async ({ page }) => {
  const fixture = await hybridsAPI(page)
  await page.setViewportSize({ width: 390, height: 844 })
  let finishLoading!: () => void
  const pending = new Promise<void>(resolve => { finishLoading = resolve })
  await page.route('**/inventory?**', async route => {
    const query = new URL(route.request().url()).searchParams.get('q')
    if (!query) await pending
    const variants = query ? [] : fixture.variants
    await route.fulfill({ json: { data: variants, total: variants.length, page: 1, page_size: 10 } })
  })

  await page.goto(`/stringers/${shopID}`)
  const book = page.getByRole('link', { name: 'Book a restring', exact: true })
  await expect(book).toBeInViewport()
  await book.click()

  const finder = page.getByRole('button', { name: 'Help me choose a complete setup' })
  await expect(finder).toBeInViewport()
  const manual = page.locator('.booking-manual')
  await expect(manual).not.toHaveAttribute('open')
  await expect(page.locator('.booking-technical')).not.toHaveAttribute('open')
  const summary = page.getByRole('complementary', { name: 'Review your setup' })
  await expect(summary).toContainText('Preview Baseline')
  await expect(summary).toContainText('Labour')
  await expect(summary.getByRole('button', { name: 'Request quote' }))
    .toHaveAccessibleDescription('Choose a string or use the guided finder.')

  await manual.locator('summary').click()
  await expect(page.getByText('Loading strings…', { exact: true })).toBeVisible()
  await expect(page.getByText('No strings match this search.', { exact: true })).toHaveCount(0)
  finishLoading()
  const firstString = page.getByRole('button', { name: /Preview String 01/ })
  await expect(firstString).toBeVisible()
  const search = page.getByRole('searchbox', { name: 'Search strings' })
  await search.fill('nothing like this')
  await expect(page.getByText('No strings match this search.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Clear search' }).click()
  await expect(search).toHaveValue('')
  await firstString.click()
  const estimate = summary.locator('.booking-total')
  const price = estimate.getByText('€15.00', { exact: true })
  await expect(price).toBeVisible()
  await page.getByRole('button', { name: /Preview String 02/ }).click()
  await expect(estimate.getByText('€29.52', { exact: true })).toBeVisible()
  await expect(summary.getByRole('button', { name: 'Request quote' })).toBeEnabled()
})
