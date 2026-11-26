import type { components } from './marketplace-contract'
export type BookingDraft = {
 acceptance?: components['schemas']['BookingAcceptance']
  tensionUnit?: 'kg' | 'lb'
  exactPounds?: true
  guided?: boolean
  sport?: 'tennis' | 'badminton' | 'squash'
  racketDetails?: string
  tensionPending?: boolean
  tensionBand?: components['schemas']['Booking']['tension_band']
  paymentMethod?: 'cash'|'onsite'|'online'
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
const generationKey = 'restringing:booking-generation'

function currentGeneration(): string | null {
  try { return localStorage.getItem(generationKey) } catch { return null }
}

// Only a newly loaded document can start a generation. Stale pages keep their original marker.
function startGeneration(): string | null {
  const existing = currentGeneration()
  if (existing) return existing
  try {
    const generation = crypto.randomUUID()
    localStorage.setItem(generationKey, generation)
    return generation
  } catch { return null }
}
const documentGeneration = startGeneration()

function discardExpiredDrafts(): void {
  try {
    const current = currentGeneration()
    for (const key of Object.keys(sessionStorage)) {
      if (!key.startsWith(prefix)) continue
      let generation: unknown
      try { generation = JSON.parse(sessionStorage.getItem(key) ?? 'null')?.generation } catch { /* Discard malformed drafts. */ }
      if (!current || generation !== current) sessionStorage.removeItem(key)
    }
  } catch { /* Storage may be blocked. */ }
}

export function watchForLogout(): void {
  discardExpiredDrafts()
  const check = () => {
    if (currentGeneration() !== documentGeneration) {
      discardExpiredDrafts()
      window.location.reload()
    }
  }
  window.addEventListener('storage', event => {
    if (event.key === generationKey || event.key === null) check()
  })
  window.addEventListener('pageshow', check)
}

export function clearBookingDrafts(): void {
  // Removing an existing marker also works when storage has reached its quota.
  try { localStorage.removeItem(generationKey) } catch { /* Storage may be blocked. */ }
  discardExpiredDrafts()
}

export function loadBookingDraft(id: string): BookingDraft | null {
  try {
    if (!documentGeneration || currentGeneration() !== documentGeneration) return null
    const value: unknown = JSON.parse(sessionStorage.getItem(prefix + id) ?? 'null')
    if (!value || typeof value !== 'object') return null
    const { generation, ...d } = value as BookingDraft & { generation?: string }
    if (generation !== documentGeneration) {
      removeBookingDraft(id)
      return null
    }
    if ((d.tensionBand !== undefined && !['lower','middle','higher','estimate'].includes(d.tensionBand)) || (d.paymentMethod !== undefined && !['cash','onsite','online'].includes(d.paymentMethod)) || (d.sport !== undefined && !['tennis','badminton','squash'].includes(d.sport)) || (d.racketDetails !== undefined && (typeof d.racketDetails !== 'string' || d.racketDetails.length > 160)) || (d.tensionPending !== undefined && typeof d.tensionPending !== 'boolean') || (d.acceptance!==undefined&&(!d.acceptance||typeof d.acceptance.version_id!=='string'||typeof d.acceptance.accepted!=='boolean'||typeof d.acceptance.early_request!=='boolean'||typeof d.acceptance.early_acknowledgement!=='boolean')) || (d.tensionUnit !== undefined && !['kg','lb'].includes(d.tensionUnit)) || (d.guided !== undefined && typeof d.guided !== 'boolean') || !Number.isFinite(d.savedAt) || Date.now() - d.savedAt > 30 * 60 * 1000 || d.savedAt > Date.now() || typeof d.racketID !== 'string' || typeof d.variantID !== 'string' || typeof d.variantModel !== 'string' || !['stock', 'customer'].includes(d.stringSource) || typeof d.mains !== 'string' || typeof d.crosses !== 'string' || typeof d.split !== 'boolean' || ![2, 4].includes(d.knots) || typeof d.stencil !== 'boolean' || (d.gripProductID !== undefined && typeof d.gripProductID !== 'string') || (d.gripLabel !== undefined && typeof d.gripLabel !== 'string') || (d.hybrid !== undefined && typeof d.hybrid !== 'boolean') || (d.crossesVariantID !== undefined && typeof d.crossesVariantID !== 'string') || (d.customerMains !== undefined && (typeof d.customerMains!=='string'||d.customerMains.length>160)) || (d.customerCrosses !== undefined && (typeof d.customerCrosses!=='string'||d.customerCrosses.length>160)) || (d.express !== undefined && typeof d.express !== 'boolean') || !Number.isInteger(d.prestretch) || typeof d.notes !== 'string' || d.notes.length > 500 || (d.attempt !== null && (typeof d.attempt?.quoteID !== 'string' || typeof d.attempt?.key !== 'string'))) return null
    return d
  } catch { return null }
}
export function saveBookingDraft(id: string, value: BookingDraft): boolean {
  if (!documentGeneration || currentGeneration() !== documentGeneration) return false
  try { sessionStorage.setItem(prefix + id, JSON.stringify({ ...value, generation: documentGeneration })); return true } catch { return false }
}
export function removeBookingDraft(id: string): void {
  if (!documentGeneration || currentGeneration() !== documentGeneration) return
  try { sessionStorage.removeItem(prefix + id) } catch { /* Browser policy may disable storage. */ }
}
