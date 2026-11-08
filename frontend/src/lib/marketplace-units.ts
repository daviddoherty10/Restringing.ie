export function centsToEuro(cents: number): number { if (!Number.isInteger(cents)) throw new Error('Invalid cents'); return cents / 100 }
export function kilogramsToCentiKg(kg: string): number | null { if (kg.trim() === '' || !Number.isFinite(Number(kg))) return null; const v=Number(kg)*100; return Number.isInteger(v) ? v : null }
export function inTensionRange(value: string, min: number, max: number, step: number): boolean { const v=kilogramsToCentiKg(value); return v!==null && step>0 && v>=min && v<=max && (v-min)%step===0 }
export function pageCount(total: number, pageSize: number): number { return Math.ceil(Math.max(0,total)/pageSize) }

export type TensionUnit = 'kg' | 'lb'
export function tensionDisplay(centiKg: number, unit: TensionUnit): string {
  if (!Number.isFinite(centiKg)) return '—'
  return unit === 'kg' ? String(centiKg / 100) : (centiKg / 45.359237).toFixed(2)
}
// The API stores integer centi-kg. Pound entry snaps to the nearest supported
// increment only within the allowed range; the UI always shows what is applied.
export function enteredTension(value: string, unit: TensionUnit, min: number, max: number, step: number): number | null {
  if (unit === 'kg') return kilogramsToCentiKg(value)
  if (!value.trim() || !Number.isFinite(Number(value)) || step <= 0) return null
  const converted = Number(value) * 45.359237
  // Half a displayed 0.01 lb permits round-tripping the displayed boundaries.
  const tolerance = 0.005 * 45.359237 + 1e-8
  if (converted < min - tolerance || converted > max + tolerance) return null
  return Math.min(max, Math.max(min, min + Math.round((converted - min) / step) * step))
}
