import type { Page } from '@playwright/test'

export async function openManualStrings(page: Page) {
  const disclosure = page.locator('.booking-manual')
  await disclosure.waitFor({ state: 'attached' })
  if (await disclosure.getAttribute('open') === null) {
    await disclosure.locator('summary').click()
  }
}
