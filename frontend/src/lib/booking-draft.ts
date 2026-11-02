export type BookingDraft = {
  guided?: boolean
  savedAt: number
  racketID: string
  variantID: string
  variantModel: string
  stringSource: 'stock' | 'customer'
  mains: string
  crosses: string
  split: boolean
  knots: 2 | 4
  customerMains?: string
  customerCrosses?: string
  hybrid?: boolean
  crossesVariantID?: string
  gripProductID?: string
  gripLabel?: string
  express?: boolean
  stencil: boolean
  prestretch: number
  notes: string
  attempt: { quoteID: string; key: string } | null
}
const prefix = 'restringing:booking:'
export function loadBookingDraft(id: string): BookingDraft | null {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(prefix + id) ?? 'null')
    if (!value || typeof value !== 'object') return null
    const d = value as BookingDraft
    if ((d.guided !== undefined && typeof d.guided !== 'boolean') || !Number.isFinite(d.savedAt) || Date.now() - d.savedAt > 30 * 60 * 1000 || d.savedAt > Date.now() || typeof d.racketID !== 'string' || typeof d.variantID !== 'string' || typeof d.variantModel !== 'string' || !['stock', 'customer'].includes(d.stringSource) || typeof d.mains !== 'string' || typeof d.crosses !== 'string' || typeof d.split !== 'boolean' || ![2, 4].includes(d.knots) || typeof d.stencil !== 'boolean' || (d.gripProductID !== undefined && typeof d.gripProductID !== 'string') || (d.gripLabel !== undefined && typeof d.gripLabel !== 'string') || (d.hybrid !== undefined && typeof d.hybrid !== 'boolean') || (d.crossesVariantID !== undefined && typeof d.crossesVariantID !== 'string') || (d.customerMains !== undefined && (typeof d.customerMains!=='string'||d.customerMains.length>160)) || (d.customerCrosses !== undefined && (typeof d.customerCrosses!=='string'||d.customerCrosses.length>160)) || (d.express !== undefined && typeof d.express !== 'boolean') || !Number.isInteger(d.prestretch) || typeof d.notes !== 'string' || d.notes.length > 500 || (d.attempt !== null && (typeof d.attempt?.quoteID !== 'string' || typeof d.attempt?.key !== 'string'))) return null
    return d
  } catch { return null }
}
export function saveBookingDraft(id: string, value: BookingDraft): boolean {
  try { sessionStorage.setItem(prefix + id, JSON.stringify(value)); return true } catch { return false }
}
export function removeBookingDraft(id: string): void {
  try { sessionStorage.removeItem(prefix + id) } catch { /* Browser policy may disable storage. */ }
}
