import type { OrderStatus } from '../../lib/marketplace-api'
export const orderStatuses: OrderStatus[] = ['placed', 'accepted', 'received', 'in_progress', 'ready', 'completed', 'cancelled']
export const statusLabel = (value: string) => value.replaceAll('_', ' ')
export const euro = (cents: number) => new Intl.NumberFormat('en-IE', { style: 'currency', currency: 'EUR' }).format(cents / 100)
export function decimalUnits(value: string, places: number): number | null {
  if (!new RegExp(`^[0-9]+(?:\\.[0-9]{1,${places}})?$`).test(value.trim())) return null
  const [whole, fraction = ''] = value.trim().split('.')
  const units = Number(whole) * 10 ** places + Number(fraction.padEnd(places, '0'))
  return Number.isSafeInteger(units) ? units : null
}
export function signedUnits(value: string, places: number): number | null {
  const negative = value.trim().startsWith('-')
  const result = decimalUnits(negative ? value.trim().slice(1) : value.trim(), places)
  return result === null ? null : negative ? -result : result
}
export function nextOrderStatus(status: string): OrderStatus | null {
  const next: Record<string, OrderStatus> = { placed: 'accepted', accepted: 'received', received: 'in_progress', in_progress: 'ready', ready: 'completed' }
  return next[status] ?? null
}
export function cancellable(status: string): boolean { return ['placed', 'accepted', 'received'].includes(status) }
