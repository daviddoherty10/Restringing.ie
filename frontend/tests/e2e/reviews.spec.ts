import { expect, test } from '@playwright/test'
import { trackingAPI } from './tracking-fixture.ts'
import { dashboardAPI, shopID, uid } from './dashboard-fixture.ts'

const review = { id: uid(9510), rating: 5, comment: 'Good turnaround' }

test('customers can review a collected racket and remove their review', async ({ page }) => {
  const { order } = await trackingAPI(page)
  let saved: typeof review | null = null
  await page.route('**/orders/*/review', async route => {
    if (route.request().method() === 'PUT') {
      expect(route.request().postDataJSON()).toEqual({ rating: 5, comment: review.comment })
      saved = review
    }
    await route.fulfill({ json: { review: saved, hidden: false } })
  })
  await page.route('**/orders/*/review/withdrawal', async route => {
    saved = null
    await route.fulfill({ json: { read: true } })
  })
  await page.goto('/orders/' + order.id)
  await expect(page.getByRole('heading', { name: 'Stringing underway' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Rate your restring' })).toHaveCount(0)
  order.status = 'completed'
  await page.getByRole('button', { name: 'Refresh order', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Collected', exact: true })).toBeVisible()
  await page.getByLabel('Rating', { exact: true }).selectOption('5')
  await page.getByLabel('Comment (optional)').fill(review.comment)
  await page.getByRole('button', { name: 'Save review', exact: true }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Review saved.' })).toBeVisible()
  await page.getByRole('button', { name: 'Remove review', exact: true }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Review removed.' })).toBeVisible()
})

test('the profile only displays reviews when the API publishes them', async ({ page }) => {
  await dashboardAPI(page)
  await page.route('**/api/v1/stringers/' + shopID, route => route.fulfill({
    json: { id: shopID, name: 'Preview Baseline', area: 'Dublin', county: 'Dublin', booking_available: true },
  }))
  await page.route('**/api/v1/stringers/' + shopID + '/services', route => route.fulfill({ json: [] }))
  let published = false
  await page.route('**/stringers/*/reviews', route => route.fulfill({
    json: published ? { count: 5, average: 4.8, reviews: [review] } : { count: 0, average: 0, reviews: [] },
  }))
  await page.goto('/stringers/' + shopID)
  await expect(page.getByText('Not enough reviews yet.')).toBeVisible()
  await expect(page.getByText('Verified booking')).toHaveCount(0)
  published = true
  await page.reload()
  await expect(page.getByText('4.8 out of 5')).toBeVisible()
  await expect(page.getByText(review.comment)).toBeVisible()
})

test('admin can hide a reported review', async ({ page }) => {
  await dashboardAPI(page, 'admin')
  let decided = false
  await page.route('**/api/v1/admin/reviews**', async route => {
    if (route.request().method() === 'PATCH') {
      expect(route.request().postDataJSON()).toEqual({ version: 2, hide: true })
      decided = true
      return route.fulfill({ json: { read: true } })
    }
    await route.fulfill({
      json: { data: decided ? [] : [{ ...review, business: 'Preview Baseline', reason: 'Personal details', version: 2 }], page: 1, page_size: 20, total: decided ? 0 : 1 },
    })
  })
  await page.goto('/admin/reviews')
  await page.getByRole('button', { name: 'Hide review', exact: true }).click()
  await expect(page.getByText('No reported reviews.')).toBeVisible()
})
