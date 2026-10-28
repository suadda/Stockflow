import { useId, useState, type FormEvent } from 'react'
import { api, type ApiError, toApiError } from '../api/client'
import type { MovementType, Product } from '../api/types'
import { validateQty } from '../lib/validate'
import { Modal } from './Modal'

interface Props {
  product: Product
  onClose: () => void
  onDone: (p: Product) => void
}

const MAX_NOTE = 500

export function AdjustStockModal({ product, onClose, onDone }: Props) {
  const base = useId()
  const [type, setType] = useState<MovementType>('in')
  const [qty, setQty] = useState('')
  const [note, setNote] = useState('')
  const [qtyTouched, setQtyTouched] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)

  const qtyError = validateQty(qty)
  const noteError = Array.from(note).length > MAX_NOTE ? `Note must be at most ${MAX_NOTE} characters` : null
  const showQtyError = qtyError !== null && (qtyTouched || qty !== '')

  // An out larger than stock is deliberately NOT blocked here: the server's
  // guarded update is the authority, and its 409 is shown below.
  async function submit(e: FormEvent) {
    e.preventDefault()
    if (qtyError || noteError || saving) return
    setSaving(true)
    setError(null)
    try {
      const trimmed = note.trim()
      const updated = await api.adjustStock(product.id, {
        type,
        qty: Number(qty),
        ...(trimmed ? { note: trimmed } : {}),
      })
      onDone(updated)
    } catch (err) {
      setError(toApiError(err))
      setSaving(false)
    }
  }

  let errorText: string | null = null
  if (error) {
    if (error.code === 'INSUFFICIENT_STOCK') {
      errorText = `Only ${error.available ?? 0} in stock. You tried to remove ${qty}.`
    } else if (error.details && error.details.length > 0) {
      errorText = error.details.map((d) => d.message).join('; ')
    } else {
      errorText = error.message
    }
  }

  return (
    <Modal title={`Adjust stock: ${product.sku}`} onClose={onClose}>
      <p className="muted">
        {product.name} · current stock <strong>{product.stock}</strong>
      </p>
      <form onSubmit={submit} noValidate>
        <fieldset className="field radio-group">
          <legend>Direction</legend>
          {(
            [
              ['in', 'Stock in (receive)'],
              ['out', 'Stock out (remove)'],
            ] as const
          ).map(([value, label]) => (
            <label key={value} className="radio">
              <input
                type="radio"
                name={`${base}-type`}
                value={value}
                checked={type === value}
                onChange={() => {
                  setType(value)
                  setError(null)
                }}
              />
              {label}
            </label>
          ))}
        </fieldset>

        <div className="field">
          <label htmlFor={`${base}-qty`}>Quantity</label>
          <input
            id={`${base}-qty`}
            inputMode="numeric"
            autoComplete="off"
            value={qty}
            onChange={(e) => {
              setQty(e.target.value)
              setError(null)
            }}
            onBlur={() => setQtyTouched(true)}
            aria-invalid={showQtyError}
            aria-describedby={showQtyError ? `${base}-qty-error` : undefined}
          />
          {showQtyError && (
            <p id={`${base}-qty-error`} className="field-error">
              {qtyError}
            </p>
          )}
        </div>

        <div className="field">
          <label htmlFor={`${base}-note`}>Note (optional)</label>
          <input
            id={`${base}-note`}
            autoComplete="off"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            aria-invalid={noteError !== null}
            aria-describedby={noteError ? `${base}-note-error` : undefined}
          />
          {noteError && (
            <p id={`${base}-note-error`} className="field-error">
              {noteError}
            </p>
          )}
        </div>

        {errorText && (
          <p className="banner banner-error" role="alert">
            {errorText}
          </p>
        )}

        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={qtyError !== null || noteError !== null || saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
