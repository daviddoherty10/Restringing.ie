export function centsToEuro(cents: number): number { if (!Number.isInteger(cents)) throw new Error('Invalid cents'); return cents / 100 }
export function kilogramsToCentiKg(kg: string): number | null { if (kg.trim() === '' || !Number.isFinite(Number(kg))) return null; const v=Number(kg)*100; return Number.isInteger(v) ? v : null }
export function inTensionRange(value: string, min: number, max: number, step: number): boolean { const v=kilogramsToCentiKg(value); return v!==null && step>0 && v>=min && v<=max && (v-min)%step===0 }
export function pageCount(total: number, pageSize: number): number { return Math.ceil(Math.max(0,total)/pageSize) }
