import { expect, test } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
import { shopID, uid } from './dashboard-fixture.ts'
test('draft legal pages retain draft warnings and working links on mobile', async ({
  page,
}) => {
  await hybridsAPI(page)
  await page.setViewportSize({ width: 320, height: 740 })
  for (const path of [
    'privacy',
    'terms',
    'cookies',
    'cancellations',
    'contact',
  ]) {
    await page.goto(`/${path}`)
    await expect(
      page.getByText('LEGAL INFORMATION — PRE-LAUNCH DRAFT', { exact: true }),
    ).toBeVisible()
    await expect(
      page.getByRole('status').filter({ hasText: 'Draft for review' }),
    ).toBeVisible()
    await expect
      .poll(() =>
        page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      )
      .toBe(true)
  }
})
test('enforced disclosures block unchecked acceptance and collect early choices separately', async ({
  page,
}) => {
  const f = await hybridsAPI(page)
  await page.route('**/checkout-disclosure', (route) =>
    route.fulfill({
      json: {
        enabled: true,
        version_id: uid(9900),
        document: {
          version: 'TEST ONLY',
          trader: {
            name: 'Fictional trader',
            address: 'Test address',
            email: 'test@example.invalid',
            phone: '+447700900000',
            trader_status: 'TEST',
          },
          terms: 'TEST ONLY terms',
          cancellation_notice: 'TEST ONLY cancellation',
          price_notice: 'TEST ONLY price notice',
          turnaround: 'TEST ONLY turnaround',
          acceptance_label: 'Accept TEST terms',
          early_request_label: 'Request TEST early performance',
          early_acknowledgement_label: 'Acknowledge TEST early consequences',
          submit_label: 'TEST place order',
          withdrawal_days: 14,
          reviewed: true,
          review_reference: 'TEST',
        },
      },
    }),
  )
  await page.goto(`/stringers/${shopID}/book`)
  await page
    .getByRole('combobox', { name: 'Racket model', exact: true })
    .selectOption(f.racket.id)
  await page.getByRole('button', { name: /Preview String 01/ }).click()
  await page.getByRole('button', { name: /Request server quote/ }).click()
  const book = page.getByRole('button', {
    name: 'TEST place order',
    exact: true,
  })
  await expect(book).toBeDisabled()
  await expect(
    page.getByRole('checkbox', { name: 'Accept TEST terms', exact: true }),
  ).not.toBeChecked()
  await page
    .getByRole('checkbox', { name: 'Accept TEST terms', exact: true })
    .check()
  await expect(book).toBeEnabled()
  await page
    .getByRole('checkbox', {
      name: 'Request TEST early performance',
      exact: true,
    })
    .check()
  await expect(book).toBeDisabled()
  await page
    .getByRole('checkbox', {
      name: 'Acknowledge TEST early consequences',
      exact: true,
    })
    .check()
  await expect(book).toBeEnabled()
  const request = page.waitForRequest(
    (r) => r.url().endsWith('/orders') && r.method() === 'POST',
  )
  await book.click()
  expect((await request).postDataJSON().acceptance).toEqual({
    version_id: uid(9900),
    accepted: true,
    early_request: true,
    early_acknowledgement: true,
  })
})
