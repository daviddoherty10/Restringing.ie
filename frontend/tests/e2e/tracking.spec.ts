import { expect, test } from '@playwright/test'
import { trackingAPI } from './tracking-fixture.ts'

test('live tracker follows real stages, preserves setup and handles cancelled orders', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  await page.goto(`/orders/${f.order.id}`)
  await expect(
    page.getByRole('heading', { name: 'Stringing underway', exact: true }),
  ).toBeVisible()
  const progress = page.getByRole('region', { name: 'Order progress' })
  await expect(progress.locator('[aria-current=step]')).toContainText(
    'Stringing underway',
  )
  await expect(progress.locator('time')).toHaveCount(4)
  await expect(
    page.getByText('Awaiting your stringer’s estimate'),
  ).toBeVisible()
  await expect(
    page.getByRole('heading', { name: 'Booking & payment' }),
  ).toBeVisible()
  await expect(page.getByText('24.5 / 24 kg', { exact: true })).toBeVisible()
  f.order.status = 'ready'
  await page.getByRole('button', { name: 'Refresh order', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Ready for collection', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByText('Your racket is ready', { exact: true }),
  ).toBeVisible()
  await expect(
    page.getByText('Cash due directly to the stringer', { exact: false }),
  ).toBeVisible()
  f.order.status = 'completed'
  await page.getByRole('button', { name: 'Refresh order', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Collected', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Request collection date', exact: true }),
  ).toHaveCount(0)
  f.order.status = 'cancelled'
  await page.getByRole('button', { name: 'Refresh order', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Order cancelled', exact: true }),
  ).toBeVisible()
  await expect(progress.locator('[aria-current=step]')).toHaveCount(0)
  await expect(
    page.getByRole('button', { name: 'Send message', exact: true }),
  ).toBeVisible()
})
test('a requested collection date is separate from the confirmed estimate and safely retries', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  f.failNextTracking()
  await page.goto(`/orders/${f.order.id}`)
  await page
    .getByLabel('Requested collection date', { exact: true })
    .fill('2027-01-10')
  await page
    .getByRole('button', { name: 'Request collection date', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Retry same update', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByLabel('Requested collection date', { exact: true }),
  ).toBeDisabled()
  await page
    .getByRole('button', { name: 'Retry same update', exact: true })
    .click()
  await expect(
    page.getByText(
      'Collection date requested. Your stringer will confirm what is possible.',
    ),
  ).toBeVisible()
  await expect(
    page.getByText('Awaiting your stringer’s estimate'),
  ).toBeVisible()
  const attempts = f.writes.filter((w) =>
    w.path.endsWith('/collection-request'),
  )
  expect(attempts).toHaveLength(2)
  expect(attempts[1]).toEqual(attempts[0])
  expect(f.updates).toHaveLength(1)
})
test('stringers can flag delays, confirm an estimate and add collection instructions', async ({
  page,
}) => {
  const f = await trackingAPI(page, true)
  await page.goto(`/orders/${f.order.id}`)
  await expect(
    page.getByRole('heading', { name: 'Update collection details' }),
  ).toBeVisible()
  await page
    .getByLabel('Confirmed ready estimate', { exact: true })
    .fill('2027-01-10T18:00')
  await page
    .getByLabel('Collection instructions', { exact: true })
    .fill('Collect at the club reception after 6pm.')
  await page.getByLabel('Flag an issue needing attention').check()
  await page
    .getByLabel('Update for the customer', { exact: true })
    .fill('There is a delay with the machine. Please reply about collection.')
  await page
    .getByRole('button', { name: 'Save collection update', exact: true })
    .click()
  await expect(
    page.getByText('Your order needs attention', { exact: true }),
  ).toBeVisible()
  await expect(
    page
      .locator('.tracking-updates')
      .getByText(
        'There is a delay with the machine. Please reply about collection.',
        { exact: true },
      ),
  ).toBeVisible()
  await expect(
    page
      .locator('.tracking-instructions')
      .filter({ hasText: 'Collect at the club' }),
  ).toContainText('Collect at the club reception')
  expect(f.tracking.confirmed_ready_at).toBeTruthy()
  expect(f.order.status).toBe('in_progress')
  expect(f.order.total_cents).toBe(2599)
})
test('an uncertain message retry keeps its key and creates one message', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  f.failNextMessage()
  await page.goto(`/orders/${f.order.id}`)
  await page
    .getByLabel('Your message', { exact: true })
    .fill('Can I collect before my match?')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(
    page.getByRole('button', { name: 'Retry same message', exact: true }),
  ).toBeVisible()
  await expect(page.getByLabel('Your message', { exact: true })).toBeDisabled()
  await page
    .getByRole('button', { name: 'Retry same message', exact: true })
    .click()
  await expect(page.getByText('Message sent.', { exact: true })).toBeVisible()
  await expect(page.locator('.message-list li')).toHaveCount(1)
  const attempts = f.writes.filter((w) => w.path.endsWith('/messages'))
  expect(attempts).toHaveLength(2)
  expect(attempts[1]).toEqual(attempts[0])
  expect(f.messages).toHaveLength(1)
})
test('inbox shows ready updates, marks read and links to tracking', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  await page.goto('/notifications')
  await expect(page.getByText('1 unread', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Mark read', exact: true }).click()
  await expect(page.getByText('0 unread', { exact: true })).toBeVisible()
  await page
    .getByRole('link', {
      name: 'Your racket is ready for collection →',
      exact: true,
    })
    .click()
  await expect(page).toHaveURL(`/orders/${f.order.id}`)
  await expect(
    page.getByRole('heading', { name: 'Stringing underway', exact: true }),
  ).toBeVisible()
})
test('tracking refreshes in place and marks only returned messages read', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  f.incomingMessage('Your racket has arrived safely.')
  await page.clock.install()
  await page.goto(`/orders/${f.order.id}`)
  await expect(
    page.getByText('Your racket has arrived safely.', { exact: true }),
  ).toBeVisible()
  await expect
    .poll(
      () => f.writes.filter((w) => w.path.endsWith('/messages/read')).length,
    )
    .toBeGreaterThan(0)
  expect(
    f.writes.find((w) => w.path.endsWith('/messages/read'))?.body
      .last_message_id,
  ).toBe(f.messages[0].id)
  f.order.status = 'ready'
  f.incomingMessage('Ready to collect at reception.')
  await page.clock.runFor(16000)
  await expect(
    page.getByRole('heading', { name: 'Ready for collection', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByText('Ready to collect at reception.', { exact: true }),
  ).toBeVisible()
})
test('stale collection edits refresh the current version without overwriting it', async ({
  page,
}) => {
  const f = await trackingAPI(page, true)
  await page.goto(`/orders/${f.order.id}`)
  await page
    .getByLabel('Update for the customer', { exact: true })
    .fill('Old draft')
  f.tracking.version = 1
  f.tracking.collection_instructions =
    'New details saved by another staff member.'
  await page
    .getByRole('button', { name: 'Save collection update', exact: true })
    .click()
  await expect(
    page.getByRole('status').filter({ hasText: 'Collection details changed.' }),
  ).toBeVisible()
  await expect(
    page.getByLabel('Update for the customer', { exact: true }),
  ).toHaveValue('Old draft')
  await page
    .getByRole('button', {
      name: 'Load latest collection details',
      exact: true,
    })
    .click()
  await expect(
    page.getByLabel('Collection instructions', { exact: true }),
  ).toHaveValue(f.tracking.collection_instructions)
  expect(f.updates).toHaveLength(0)
})
test('small screens show the entire tracker and plain text messages without overflow', async ({
  page,
}, info) => {
  const f = await trackingAPI(page)
  f.tracking.issue_open = true
  f.tracking.confirmed_ready_at = '2020-01-01T18:00:00Z'
  f.tracking.collection_instructions =
    'Collect from the club reception. Message us when you arrive.'
  f.incomingMessage('<script>alert("not HTML")</script> ' + 'x'.repeat(200))
  await page.setViewportSize({ width: 320, height: 700 })
  await page.goto(`/orders/${f.order.id}`)
  await expect(
    page.getByText('Your order needs attention', { exact: true }),
  ).toBeVisible()
  await expect(
    page.getByText(
      'The estimate has passed. Ask your stringer for an update below.',
    ),
  ).toBeVisible()
  await expect(page.locator('.message-list')).toContainText('<script>')
  await expect(page.locator('.message-list script')).toHaveCount(0)
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true)
  await page.screenshot({
    path: `/tmp/restringing-tracking-${info.project.name}.png`,
    fullPage: true,
  })
})
test('an inaccessible order reveals no tracking or messages', async ({
  page,
}) => {
  const f = await trackingAPI(page)
  await page.route(`**/api/v1/orders/${f.order.id}`, (route) =>
    route.fulfill({
      status: 404,
      json: {
        error: {
          code: 'not_found',
          message: 'The requested order was not found.',
        },
      },
    }),
  )
  await page.goto(`/orders/${f.order.id}`)
  await expect(page.getByRole('alert')).toContainText(
    'The requested order was not found.',
  )
  await expect(
    page.getByRole('heading', { name: 'Order conversation', exact: true }),
  ).toHaveCount(0)
  await expect(page.locator('.tracking-hero')).toHaveCount(0)
})
