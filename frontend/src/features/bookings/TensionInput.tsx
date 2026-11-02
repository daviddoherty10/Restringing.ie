import { useId, useState } from 'react'
import {
  enteredTension,
  kilogramsToCentiKg,
  tensionDisplay,
  type TensionUnit,
} from '../../lib/marketplace-units'
export default function TensionInput({
  label,
  value,
  unit,
  min,
  max,
  step,
  change,
}: {
  label: string
  value: string
  unit: TensionUnit
  min: number
  max: number
  step: number
  change: (kg: string) => void
}) {
  const hint = useId()
  const [entry, setEntry] = useState({ canonical: value, unit, text: '' })
  const centi = kilogramsToCentiKg(value)
  const shown =
    entry.canonical === value && entry.unit === unit && entry.text !== ''
      ? entry.text
      : centi === null
        ? ''
        : tensionDisplay(centi, unit)
  return (
    <div className="booking-tension">
      <label className="booking-field">
        {label} ({unit})
        <input
          type="text"
          inputMode="decimal"
          value={shown}
          aria-describedby={hint}
          onChange={(e) => {
            const text = e.target.value,
              n = enteredTension(text, unit, min, max, step),
              canonical = n === null ? '' : String(n / 100)
            setEntry({ canonical, unit, text })
            change(canonical)
          }}
        />
      </label>
      <small id={hint}>
        {unit === 'lb'
          ? centi === null
            ? `Enter ${tensionDisplay(min, 'lb')}–${tensionDisplay(max, 'lb')} lb.`
            : `Applied: ${tensionDisplay(centi, 'kg')} kg (${tensionDisplay(centi, 'lb')} lb). Pound entry rounds to the nearest supported ${step / 100} kg increment.`
          : `Allowed: ${min / 100}–${max / 100} kg in ${step / 100} kg increments.`}
      </small>
    </div>
  )
}
