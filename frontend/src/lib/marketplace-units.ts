export function centsToEuro(cents: number): number { if (!Number.isInteger(cents)) throw new Error('Invalid cents'); return cents / 100 }
export function kilogramsToCentiKg(kg: string): number | null { if (kg.trim() === '' || !Number.isFinite(Number(kg))) return null; const v=Number(kg)*100, rounded=Math.round(v); return Number.isSafeInteger(rounded) && Math.abs(v-rounded)<1e-8 ? rounded : null }
export function inTensionRange(value: string, min: number, max: number, step: number): boolean { const v=kilogramsToCentiKg(value); return v!==null && step>0 && v>=min && v<=max && (v-min)%step===0 }
export function pageCount(total: number, pageSize: number): number { return Math.ceil(Math.max(0,total)/pageSize) }

export type TensionUnit = 'kg' | 'lb'
export function tensionDisplay(centiKg: number, unit: TensionUnit): string {
  if (!Number.isFinite(centiKg)) return '—'
  return unit === 'kg' ? String(centiKg / 100) : (centiKg / 45.359237).toFixed(2)
}
export function enteredTension(value: string, unit: TensionUnit, min: number, max: number, step: number): number | null {
  const n = kilogramsToCentiKg(value)
  if (n === null) return null
  if (unit === 'kg') return inTensionRange(value, min, max, step) ? n : null
  return n * 0.45359237 >= min && n * 0.45359237 <= max ? n : null
}

export function savedTension(b: {
  mains_tension_centi_kg: number
  crosses_tension_centi_kg: number
  mains_tension_centi_lb?: number
  crosses_tension_centi_lb?: number
}): { mains: number; crosses: number; unit: TensionUnit } {
  if (b.mains_tension_centi_lb !== undefined && b.crosses_tension_centi_lb !== undefined) {
    return { mains: b.mains_tension_centi_lb / 100, crosses: b.crosses_tension_centi_lb / 100, unit: 'lb' }
  }
  return { mains: b.mains_tension_centi_kg / 100, crosses: b.crosses_tension_centi_kg / 100, unit: 'kg' }
}

export function savedTensionLabel(b: Parameters<typeof savedTension>[0]): string {
  const { mains, crosses, unit } = savedTension(b)
  return `${mains} / ${crosses} ${unit}`
}
