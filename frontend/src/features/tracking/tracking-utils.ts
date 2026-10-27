import type { OrderStatus } from '../../lib/marketplace-api'

export const stages: {
  status: OrderStatus
  title: string
  description: string
}[] = [
  {
    status: 'placed',
    title: 'Order placed',
    description: 'Waiting for your stringer to confirm.',
  },
  {
    status: 'accepted',
    title: 'Booking confirmed',
    description: 'Your stringer has accepted your booking.',
  },
  {
    status: 'received',
    title: 'Racket received',
    description: 'Your racket has been checked in.',
  },
  {
    status: 'in_progress',
    title: 'Stringing underway',
    description: 'Your racket is being restrung.',
  },
  {
    status: 'ready',
    title: 'Ready for collection',
    description: 'Your racket is ready to get back on court.',
  },
  {
    status: 'completed',
    title: 'Collected',
    description: 'All done. Enjoy your next match.',
  },
]
export const dateTime = (value: string) =>
  new Intl.DateTimeFormat('en-IE', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'Europe/Dublin',
  }).format(new Date(value))
export const collectionDate = (value: string) =>
  new Intl.DateTimeFormat('en-IE', {
    dateStyle: 'medium',
    timeZone: 'UTC',
  }).format(new Date(`${value}T12:00:00Z`))
export function irishToday(now = new Date()) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Europe/Dublin',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(now)
  const get = (key: string) => parts.find((p) => p.type === key)!.value
  return `${get('year')}-${get('month')}-${get('day')}`
}
export function localDateTime(value: string | null) {
  if (!value) return ''
  const d = new Date(value),
    pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
