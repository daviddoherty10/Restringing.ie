export type StringMaterial = 'Multifilament' | 'Synthetic gut' | 'Polyester' | 'Natural gut'
export type StringProduct = { id: string; brand: string; name: string; material: StringMaterial; gauge: string; supplement: number; comfort: number; durability: number; control: number }
// Fictional demonstration products, not a real brand catalogue or stock records.
const materials: StringMaterial[] = ['Multifilament','Synthetic gut','Polyester','Natural gut']
export const catalogue: StringProduct[] = Array.from({ length: 28 }, (_, index) => {
  const material = materials[index % materials.length]
  const variant = Math.floor(index / materials.length) + 1
  return {
    id: 'sample-' + (index + 1),
    brand: 'Demo Strings',
    name: material + ' Series ' + String(variant).padStart(2, '0'),
    material,
    gauge: ['1.20 mm','1.25 mm','1.30 mm','1.35 mm'][variant % 4],
    supplement: ({ 'Multifilament': 13, 'Synthetic gut': 8, 'Polyester': 12, 'Natural gut': 29 } as Record<StringMaterial, number>)[material] + variant,
    comfort: material === 'Multifilament' || material === 'Natural gut' ? 3 : material === 'Synthetic gut' ? 2 : 1,
    durability: material === 'Polyester' ? 3 : 2,
    control: material === 'Polyester' ? 3 : 2,
  }
})
export const money = (amount: number) => new Intl.NumberFormat('en-IE',{style:'currency',currency:'EUR'}).format(amount)
