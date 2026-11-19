import { useState, type ChangeEvent } from 'react'
import { imageUploadData } from '../../lib/image-upload'
import { marketplaceAPI, type CatalogueVariant } from '../../lib/marketplace-api'
import { Feedback } from '../management/dashboard-shared'
import { errorMessage } from '../management/use-remote'
import '../strings/string-photo.css'

export default function VariantPhotoEditor({ variant, disabled, onBusy }: {
  variant: CatalogueVariant
  disabled: boolean
  onBusy: (busy: boolean) => void
}) {
  const [hasPhoto, setHasPhoto] = useState(!!variant.has_photo)
  const [revision, setRevision] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  async function change(file?: File) {
    if (busy || disabled) return
    setError('')
    setMessage('')
    if (file && (file.size > 1048576 || !['image/png', 'image/jpeg'].includes(file.type))) {
      setError('Choose a PNG or JPEG up to 1 MB.')
      return
    }
    setBusy(true)
    onBusy(true)
    try {
      if (file) await marketplaceAPI.admin.saveVariantPhoto(variant.id, variant.colour, await imageUploadData(file))
      else await marketplaceAPI.admin.deleteVariantPhoto(variant.id)
      setHasPhoto(!!file)
      setRevision(value => value + 1)
      setMessage(file ? 'String photo saved.' : 'String photo removed.')
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
      onBusy(false)
    }
  }

  function choose(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) void change(file)
  }

  return <section>
    <h3>String photo</h3>
    <p>Use a photo you have permission to publish that shows this exact colour{variant.colour ? ` (${variant.colour})` : ''}. PNG or JPEG, up to 1 MB.</p>
    {hasPhoto && <img className="string-photo-preview" src={marketplaceAPI.admin.variantPhotoURL(variant.id) + `?updated=${revision}`} alt={variant.colour ? `${variant.colour} string` : 'String variant photo'} width={128} height={128} />}
    <fieldset disabled={disabled || busy}>
      <legend>{busy ? 'Updating photo…' : 'Photo controls'}</legend>
      <label>Upload string photo<input type="file" accept="image/png,image/jpeg" onChange={choose} /></label>
      {hasPhoto && <button type="button" className="manage-secondary" onClick={() => void change()}>Remove string photo</button>}
    </fieldset>
    <Feedback error={error} message={message} />
  </section>
}
