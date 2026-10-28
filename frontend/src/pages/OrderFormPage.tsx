import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { api, isAbort, toApiError } from '../api/client'
import type { Product } from '../api/types'
import { formatRupiah } from '../lib/format'
import { navigate } from '../lib/useHashRoute'
import { useRefresh } from '../lib/refresh'
import {
  hasDraftErrors,
  mapValidationDetails,
  skuKey,
  toOrderRequest,
  validateOrderDraft,
  validateQty,
  type OrderDraft,
} from '../lib/validate'

interface Row {
  key: number
  sku: string
  qty: string
}

interface ServerState {
  conflict: { sku: string; available: number } | null
  fields: Record<string, string[]>
  messages: string[]
}

const NO_SERVER: ServerState = { conflict: null, fields: {}, messages: [] }

export default function OrderFormPage() {
  const { bump } = useRefresh()
  const base = useId()
  const nextKey = useRef(1)
  const newRow = (): Row => ({ key: nextKey.current++, sku: '', qty: '1' })

  const [customer, setCustomer] = useState('')
  const [rows, setRows] = useState<Row[]>(() => [newRow()])
  const [touched, setTouched] = useState<Set<string>>(new Set())
  const [saving, setSaving] = useState(false)
  const [server, setServer] = useState<ServerState>(NO_SERVER)
  const [catalog, setCatalog] = useState<Map<string, Product>>(new Map())

  // SKU suggestions and live price hints. ponytail: first 100 active products
  // only; a typeahead against ?search= is the upgrade if the catalog grows.
  useEffect(() => {
    const ctrl = new AbortController()
    api
      .listProducts({ status: 'active', limit: 100 }, ctrl.signal)
      .then((res) => setCatalog(new Map(res.data.map((p) => [p.sku, p]))))
      .catch((err: unknown) => {
        if (!isAbort(err)) setCatalog(new Map()) // hints are optional
      })
    return () => ctrl.abort()
  }, [])

  const draft: OrderDraft = { customer, rows: rows.map((r) => ({ sku: r.sku, qty: r.qty })) }
  const errs = validateOrderDraft(draft)
  const invalid = hasDraftErrors(errs)

  const touch = (k: string) => setTouched((prev) => new Set(prev).add(k))
  const shown = (k: string, msg: string | undefined) => (touched.has(k) ? msg : undefined)

  // Any edit makes earlier server feedback stale.
  const edit = (fn: () => void) => {
    fn()
    setServer(NO_SERVER)
  }
  const updateRow = (key: number, patch: Partial<Row>) =>
    edit(() => setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...patch } : r))))

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (invalid || saving) return
    setSaving(true)
    setServer(NO_SERVER)
    try {
      const order = await api.createOrder(toOrderRequest(draft))
      bump()
      navigate(`#/orders/${order.id}`)
    } catch (err) {
      const ae = toApiError(err)
      if (ae.code === 'INSUFFICIENT_STOCK' && ae.sku !== undefined) {
        setServer({ ...NO_SERVER, conflict: { sku: ae.sku, available: ae.available ?? 0 } })
      } else if (ae.code === 'VALIDATION_ERROR' && ae.details) {
        const { fields, unmatched } = mapValidationDetails(ae.details)
        setServer({ conflict: null, fields, messages: [...(fields.items ?? []), ...unmatched] })
      } else {
        setServer({ ...NO_SERVER, messages: [ae.message] })
      }
      setSaving(false)
    }
  }

  // Client-side estimate only; the server computes the real total.
  let estimate: number | null = 0
  for (const r of rows) {
    const p = catalog.get(r.sku.trim())
    if (!p || validateQty(r.qty) !== null) {
      estimate = null
      break
    }
    estimate += p.price * Number(r.qty)
  }

  const customerMsgs = [shown('customer', errs.customer), ...(server.fields.customer ?? [])].filter(
    (m): m is string => m !== undefined,
  )
  const formMsgs = [...(errs.form ? [errs.form] : []), ...server.messages]

  return (
    <section aria-labelledby="order-form-title">
      <h1 id="order-form-title">New order</h1>
      <form className="order-form" onSubmit={submit} noValidate>
        <div className="field">
          <label htmlFor={`${base}-customer`}>Customer</label>
          <input
            id={`${base}-customer`}
            value={customer}
            onChange={(e) => edit(() => setCustomer(e.target.value))}
            onBlur={() => touch('customer')}
            autoComplete="off"
            aria-invalid={customerMsgs.length > 0}
            aria-describedby={customerMsgs.length > 0 ? `${base}-customer-error` : undefined}
          />
          {customerMsgs.length > 0 && (
            <p id={`${base}-customer-error`} className="field-error">
              {customerMsgs.join('; ')}
            </p>
          )}
        </div>

        <datalist id={`${base}-skus`}>
          {[...catalog.values()].map((p) => (
            <option key={p.id} value={p.sku}>
              {p.name}
            </option>
          ))}
        </datalist>

        <h2 className="section-title">Items</h2>
        {server.conflict && (
          <p className="banner banner-error" role="alert">
            Not enough stock for {server.conflict.sku}: only {server.conflict.available} available. Nothing was
            ordered.
          </p>
        )}

        <ol className="order-rows">
          {rows.map((r, i) => {
            const rowErr = errs.rows[i]
            const skuMsgs = [shown(`${r.key}-sku`, rowErr?.sku), ...(server.fields[`items[${i}].sku`] ?? [])].filter(
              (m): m is string => m !== undefined,
            )
            const qtyMsgs = [shown(`${r.key}-qty`, rowErr?.qty), ...(server.fields[`items[${i}].qty`] ?? [])].filter(
              (m): m is string => m !== undefined,
            )
            const conflict =
              server.conflict !== null && skuKey(r.sku) === skuKey(server.conflict.sku) ? server.conflict : null
            const info = catalog.get(r.sku.trim())
            const id = `${base}-row-${r.key}`
            return (
              <li key={r.key} className={`order-row${conflict ? ' conflict' : ''}`}>
                <div className="field">
                  <label htmlFor={`${id}-sku`}>
                    <span className="sr-only">Item {i + 1} </span>SKU
                  </label>
                  <input
                    id={`${id}-sku`}
                    list={`${base}-skus`}
                    value={r.sku}
                    onChange={(e) => updateRow(r.key, { sku: e.target.value })}
                    onBlur={() => touch(`${r.key}-sku`)}
                    autoComplete="off"
                    aria-invalid={skuMsgs.length > 0 || conflict !== null}
                    aria-describedby={`${id}-sku-msg`}
                  />
                  <p id={`${id}-sku-msg`} className={skuMsgs.length > 0 || conflict ? 'field-error' : 'hint'}>
                    {conflict
                      ? `Only ${conflict.available} available`
                      : skuMsgs.length > 0
                        ? skuMsgs.join('; ')
                        : info
                          ? `${info.name} · ${formatRupiah(info.price)} · ${info.stock} in stock`
                          : '\u00a0'}
                  </p>
                </div>
                <div className="field qty">
                  <label htmlFor={`${id}-qty`}>
                    <span className="sr-only">Item {i + 1} </span>Qty
                  </label>
                  <input
                    id={`${id}-qty`}
                    inputMode="numeric"
                    value={r.qty}
                    onChange={(e) => updateRow(r.key, { qty: e.target.value })}
                    onBlur={() => touch(`${r.key}-qty`)}
                    autoComplete="off"
                    aria-invalid={qtyMsgs.length > 0}
                    aria-describedby={qtyMsgs.length > 0 ? `${id}-qty-error` : undefined}
                  />
                  {qtyMsgs.length > 0 && (
                    <p id={`${id}-qty-error`} className="field-error">
                      {qtyMsgs.join('; ')}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  className="btn btn-sm remove-row"
                  onClick={() => edit(() => setRows((rs) => rs.filter((x) => x.key !== r.key)))}
                  aria-label={`Remove item ${i + 1}`}
                >
                  Remove
                </button>
              </li>
            )
          })}
        </ol>

        <button type="button" className="btn" onClick={() => edit(() => setRows((rs) => [...rs, newRow()]))}>
          Add item
        </button>

        {formMsgs.length > 0 && (
          <ul className="banner banner-error form-errors" role="alert">
            {formMsgs.map((m, i) => (
              <li key={i}>{m}</li>
            ))}
          </ul>
        )}

        <div className="order-submit">
          <p className="muted">
            {estimate !== null && rows.length > 0
              ? `Estimated total ${formatRupiah(estimate)} (final total is computed by the server)`
              : invalid
                ? 'Fill in the customer and every item to create the order.'
                : '\u00a0'}
          </p>
          <button type="submit" className="btn btn-primary" disabled={invalid || saving}>
            {saving ? 'Creating…' : 'Create order'}
          </button>
        </div>
      </form>
    </section>
  )
}
