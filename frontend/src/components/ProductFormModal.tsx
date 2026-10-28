import { useId, useState, type ChangeEvent, type FormEvent } from 'react'
import { api, toApiError } from '../api/client'
import type { Product } from '../api/types'
import { validateProductForm, type ProductFormValues } from '../lib/validate'
import { Modal } from './Modal'

interface Props {
  /** Absent for create, present for edit. */
  product?: Product
  onClose: () => void
  onSaved: (p: Product) => void
}

type FieldKey = keyof ProductFormValues

export function ProductFormModal({ product, onClose, onSaved }: Props) {
  const base = useId()
  const [values, setValues] = useState<ProductFormValues>({
    sku: product?.sku ?? '',
    name: product?.name ?? '',
    price: product ? String(product.price) : '',
  })
  const [active, setActive] = useState(product?.is_active ?? true)
  const [submitted, setSubmitted] = useState(false)
  const [saving, setSaving] = useState(false)
  const [serverErrors, setServerErrors] = useState<Partial<Record<FieldKey, string>>>({})
  const [formError, setFormError] = useState<string | null>(null)

  const clientErrors = validateProductForm(values)
  const errorFor = (k: FieldKey) => serverErrors[k] ?? (submitted ? clientErrors[k] : undefined)

  const onChange = (k: FieldKey) => (e: ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value
    setValues((v) => ({ ...v, [k]: value }))
    setServerErrors((prev) => {
      const next = { ...prev }
      delete next[k]
      return next
    })
  }

  const fieldProps = (k: FieldKey) => {
    const err = errorFor(k)
    return {
      id: `${base}-${k}`,
      value: values[k],
      onChange: onChange(k),
      'aria-invalid': err !== undefined,
      'aria-describedby': err !== undefined ? `${base}-${k}-error` : undefined,
    }
  }

  const fieldError = (k: FieldKey) => {
    const err = errorFor(k)
    return err === undefined ? null : (
      <p id={`${base}-${k}-error`} className="field-error">
        {err}
      </p>
    )
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSubmitted(true)
    setFormError(null)
    if (Object.keys(clientErrors).length > 0) return

    setSaving(true)
    const name = values.name.trim()
    const price = Number(values.price)
    try {
      const saved = product
        ? await api.updateProduct(product.id, { name, price, is_active: active })
        : await api.createProduct({ sku: values.sku.trim(), name, price, is_active: active })
      onSaved(saved)
    } catch (err) {
      const ae = toApiError(err)
      if (ae.code === 'SKU_EXISTS') {
        setServerErrors({ sku: 'A product with this SKU already exists' })
      } else if (ae.code === 'VALIDATION_ERROR' && ae.details && ae.details.length > 0) {
        const fields: Partial<Record<FieldKey, string>> = {}
        const rest: string[] = []
        for (const d of ae.details) {
          if (d.field === 'sku' || d.field === 'name' || d.field === 'price') fields[d.field] = d.message
          else rest.push(d.message)
        }
        setServerErrors(fields)
        if (rest.length > 0) setFormError(rest.join('; '))
      } else {
        setFormError(ae.message)
      }
      setSaving(false)
    }
  }

  return (
    <Modal title={product ? `Edit ${product.sku}` : 'New product'} onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <div className="field">
          <label htmlFor={`${base}-sku`}>SKU</label>
          <input {...fieldProps('sku')} readOnly={product !== undefined} autoComplete="off" />
          {product && <p className="hint">SKU cannot be changed after creation.</p>}
          {fieldError('sku')}
        </div>
        <div className="field">
          <label htmlFor={`${base}-name`}>Name</label>
          <input {...fieldProps('name')} autoComplete="off" />
          {fieldError('name')}
        </div>
        <div className="field">
          <label htmlFor={`${base}-price`}>Price (IDR, whole rupiah)</label>
          <input {...fieldProps('price')} inputMode="numeric" autoComplete="off" />
          {fieldError('price')}
        </div>
        <div className="field field-check">
          <input id={`${base}-active`} type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
          <label htmlFor={`${base}-active`}>Active (available for orders)</label>
        </div>
        {!product && <p className="hint">New products start with stock 0. Use Adjust to receive stock.</p>}

        {formError && (
          <p className="banner banner-error" role="alert">
            {formError}
          </p>
        )}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
