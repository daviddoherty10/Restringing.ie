export type Stringer = {
  id: number
  name: string
  area: string
  county: string
  experience: string
  fromPrice: number
  turnaround: string
  racketTypes: string[]
  initials: string
}

export const examples: Stringer[] = [
  { id: 1, name: 'Baseline Stringing', area: 'Dún Laoghaire', county: 'Dublin', experience: 'Tennis specialist', fromPrice: 20, turnaround: '2–3 days', racketTypes: ['Tennis'], initials: 'BS' },
  { id: 2, name: 'The Racket Workshop', area: 'Blackrock', county: 'Dublin', experience: 'Tennis & badminton', fromPrice: 22, turnaround: '1–2 days', racketTypes: ['Tennis', 'Badminton'], initials: 'RW' },
  { id: 3, name: 'Court Craft', area: 'Sandyford', county: 'Dublin', experience: 'All racket sports', fromPrice: 18, turnaround: '3–4 days', racketTypes: ['Tennis', 'Squash', 'Badminton'], initials: 'CC' },
  { id: 4, name: 'Match Point Strings', area: 'Galway City', county: 'Galway', experience: 'Tennis specialist', fromPrice: 24, turnaround: '2–3 days', racketTypes: ['Tennis'], initials: 'MP' },
  { id: 5, name: 'String Lab', area: 'Cork City', county: 'Cork', experience: 'Tennis & squash', fromPrice: 21, turnaround: '1–2 days', racketTypes: ['Tennis', 'Squash'], initials: 'SL' },
  { id: 6, name: 'Rally Ready', area: 'Limerick City', county: 'Limerick', experience: 'Tennis & badminton', fromPrice: 19, turnaround: '2–4 days', racketTypes: ['Tennis', 'Badminton'], initials: 'RR' },
]

