import { useState } from 'react'
import { marketplaceAPI } from '../../lib/marketplace-api'
import './string-photo.css'

type VariantPhoto = { id: string; has_photo?: boolean }

export default function StringPhoto({ variant }: { variant: VariantPhoto }) {
  const [failed, setFailed] = useState(false)
  if (!variant.has_photo || failed) return null
  return <img className="string-photo" src={marketplaceAPI.variantPhotoURL(variant.id)} alt="" width={56} height={56} loading="lazy" onError={() => setFailed(true)} />
}
