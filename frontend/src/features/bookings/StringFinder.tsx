import { useState } from 'react'
import { catalogue, type StringProduct } from './sampleCatalogue'

type Props = { onChoose: (product: StringProduct) => void; onClose: () => void }
type Experience = 'new' | 'club' | 'advanced'
type Priority = 'comfort' | 'control' | 'durability'
export default function StringFinder({ onChoose, onClose }: Props) {
  const [experience, setExperience] = useState<Experience>('club')
  const [priority, setPriority] = useState<Priority>('comfort')
  const [sensitivity, setSensitivity] = useState(false)
  const [showMatches, setShowMatches] = useState(false)
  const ranked = [...catalogue].sort((a,b) => {
    const score = (item: StringProduct) => (sensitivity ? item.comfort * 5 : 0) + item[priority] * 4 + (experience === 'new' ? item.comfort * 3 : 0) - (sensitivity && item.material === 'Polyester' ? 10 : 0)
    return score(b)-score(a)
  }).slice(0,3)
  return <section className="finder" aria-label="String finder">
    <div className="finder-top"><h3>Help me choose a string</h3><button type="button" onClick={onClose} aria-label="Close string finder">✕</button></div>
    <p>Tell us how you play. This demonstration matches broad string characteristics to your preferences — it isn't a medical assessment or a personalised fitting.</p>
    <label className="booking-field">Playing experience<select value={experience} onChange={e=>{setExperience(e.target.value as Experience);setShowMatches(false)}}><option value="new">New / recreational</option><option value="club">Club / regular</option><option value="advanced">Advanced / competitive</option></select></label>
    <label className="booking-field">What matters most?<select value={priority} onChange={e=>{setPriority(e.target.value as Priority);setShowMatches(false)}}><option value="comfort">Comfort / feel</option><option value="control">Control</option><option value="durability">Durability</option></select></label>
    <label className="finder-check"><input type="checkbox" checked={sensitivity} onChange={e=>{setSensitivity(e.target.checked);setShowMatches(false)}} /> I have arm discomfort or an injury concern and want to prioritise comfort</label>
    {sensitivity && <p className="finder-safety">String selection can't diagnose, treat or prevent an injury. Speak to a qualified healthcare professional about pain, and check your setup with an experienced stringer. We don't ask for or store medical details in this preview.</p>}
    <button type="button" className="finder-submit" onClick={()=>setShowMatches(true)}>Show example matches →</button>
    {showMatches && <div className="finder-results" aria-live="polite"><h4>Example matches</h4><p>These are fictional products ranked by simple category scores, not clinical evidence or verified stock.</p>{ranked.map((item)=><div className="finder-result" key={item.id}><div><strong>{item.name}</strong><small>{item.material} · {item.gauge}</small></div><button type="button" onClick={()=>onChoose(item)}>Select</button></div>)}</div>}
  </section>
}
