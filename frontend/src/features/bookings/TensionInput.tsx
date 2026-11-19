import { useId } from 'react'
import type { TensionUnit } from '../../lib/marketplace-units'

export default function TensionInput({ label, value, unit, min, max, step, change }: {
  label: string
  value: string
  unit: TensionUnit
  min: number
  max: number
  step: number
  change: (value: string) => void
}) {
  const hint = useId()
  return (
    <div className="booking-tension">
      <label className="booking-field">
        {label} ({unit})
        <input
          type="text"
          inputMode="decimal"
          value={value}
          aria-describedby={hint}
          onChange={(e) => change(e.target.value)}
        />
      </label>
      <small id={hint}>
        {unit === 'lb'
          ? `Allowed: ${Math.ceil(min / 0.45359237) / 100}–${Math.floor(max / 0.45359237) / 100} lb. Your stringer receives the exact pounds you enter.`
          : `Allowed: ${min / 100}–${max / 100} kg in ${step / 100} kg increments.`}
      </small>
    </div>
  )
}
