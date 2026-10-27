import type { Page } from '@playwright/test'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
import { dashboardAPI, uid } from './dashboard-fixture.ts'
type S = components['schemas']
export async function trackingAPI(page: Page, manager = false) {
  const dashboard = await dashboardAPI(page)
  const order = dashboard.order
  order.status = 'in_progress'
  order.events = ['placed', 'accepted', 'received', 'in_progress'].map(
    (status, i) => ({
      from_status: i ? ['placed', 'accepted', 'received'][i - 1] : null,
      to_status: status,
      created_at: new Date(Date.now() - (4 - i) * 3600000).toISOString(),
    }),
  )
  const tracking: S['OrderTracking'] = {
    requested_collection_date: null,
    confirmed_ready_at: null,
    collection_instructions: '',
    issue_open: false,
    version: 0,
    stringer_name: 'Preview Baseline',
    area: 'Dun Laoghaire',
    county: 'Dublin',
    can_manage: manager,
    is_customer: !manager,
    timezone: 'Europe/Dublin',
  }
  const updates: S['TrackingUpdate'][] = [],
    messages: S['OrderMessage'][] = []
  const notifications: S['Notification'][] = [
    {
      id: uid(94001),
      order_id: order.id,
      stringer_id: order.stringer_id,
      for_stringer: manager,
      kind: 'status',
      title: 'Your racket is ready for collection',
      created_at: new Date().toISOString(),
      read_at: null,
    },
  ]
  const writes: {
    path: string
    key: string | undefined
    body: Record<string, unknown>
  }[] = []
  const keys = new Map<string, unknown>()
  let unread = 0,
    messageFailAfterCommit = false,
    trackingFailAfterCommit = false
  await page.route('**/api/v1/**', async (route) => {
    const r = route.request(),
      url = new URL(r.url()),
      path = url.pathname.replace('/api/v1', ''),
      method = r.method()
    const json = (body: unknown, status = 200) =>
      route.fulfill({ status, json: body })
    const paginate = <T>(items: T[]) => {
      const p = Number(url.searchParams.get('page') || 1),
        size = Number(url.searchParams.get('page_size') || 20)
      return {
        data: items.slice((p - 1) * size, p * size),
        page: p,
        page_size: size,
        total: items.length,
      }
    }
    if (method !== 'GET')
      writes.push({
        path,
        key: r.headers()['idempotency-key'],
        body: r.postDataJSON() ?? {},
      })
    if (path === '/me') return json({ id: uid(manager ? 4 : 1), admin: false })
    if (path === `/orders/${order.id}`) return json(order)
    if (path === '/orders') return json(paginate([order]))
    if (path === `/orders/${order.id}/tracking`) return json(tracking)
    if (path === `/orders/${order.id}/tracking-updates` && method === 'GET')
      return json(paginate([...updates].reverse()))
    if (
      method === 'POST' &&
      (path.endsWith('/tracking-updates') ||
        path.endsWith('/collection-request'))
    ) {
      const key = r.headers()['idempotency-key']!,
        body = r.postDataJSON()
      if (keys.has(key)) return json(keys.get(key))
      if (body.expected_version !== tracking.version)
        return json(
          {
            error: {
              code: 'tracking_changed',
              message:
                'Collection details changed. Refresh them before saving again.',
            },
          },
          409,
        )
      if (path.endsWith('/collection-request'))
        tracking.requested_collection_date = body.date
      else
        Object.assign(tracking, {
          confirmed_ready_at: body.confirmed_ready_at,
          collection_instructions: body.collection_instructions,
          issue_open: body.issue_open,
        })
      tracking.version++
      const u: S['TrackingUpdate'] = {
        id: uid(95000 + updates.length),
        kind: path.endsWith('/collection-request')
          ? 'collection_request'
          : 'stringer_update',
        note:
          body.note ||
          `Customer requested collection on ${body.date}. Awaiting the stringer's agreement.`,
        state: { ...tracking },
        created_at: new Date().toISOString(),
      }
      updates.push(u)
      tracking.latest_update = u
      keys.set(key, u)
      if (trackingFailAfterCommit) {
        trackingFailAfterCommit = false
        return route.abort('failed')
      }
      return json(u)
    }
    if (path === `/orders/${order.id}/messages` && method === 'GET')
      return json({
        ...paginate([...messages].reverse()),
        unread_count: unread,
      })
    if (path.endsWith('/messages/read')) {
      unread = 0
      return json({ read: true })
    }
    if (path === `/orders/${order.id}/messages` && method === 'POST') {
      const key = r.headers()['idempotency-key']!
      if (keys.has(key)) return json(keys.get(key), 201)
      const m: S['OrderMessage'] = {
        id: uid(96000 + messages.length),
        position: String(messages.length + 1),
        sender_role: manager ? 'stringer' : 'customer',
        own: true,
        body: r.postDataJSON().body,
        created_at: new Date().toISOString(),
      }
      messages.push(m)
      keys.set(key, m)
      if (messageFailAfterCommit) {
        messageFailAfterCommit = false
        return route.abort('failed')
      }
      return json(m, 201)
    }
    if (path === '/notifications')
      return json({
        ...paginate(notifications),
        unread_count: notifications.filter((n) => !n.read_at).length,
      })
    if (path.startsWith('/notifications/') && path.endsWith('/read')) {
      notifications[0].read_at = new Date().toISOString()
      return json({ read: true })
    }
    return route.fallback()
  })
  return {
    order,
    tracking,
    updates,
    messages,
    notifications,
    writes,
    failNextMessage: () => {
      messageFailAfterCommit = true
    },
    failNextTracking: () => {
      trackingFailAfterCommit = true
    },
    incomingMessage: (body: string) => {
      messages.push({
        id: uid(96000 + messages.length),
        position: String(messages.length + 1),
        sender_role: manager ? 'customer' : 'stringer',
        own: false,
        body,
        created_at: new Date().toISOString(),
      })
      unread++
    },
  }
}
