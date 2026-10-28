import { useEffect, useState } from 'react'
import { api, type ApiError, isAbort, toApiError } from '../api/client'
import type { Order } from '../api/types'
import { ErrorState } from '../components/ErrorState'
import { formatDateTime, formatRupiah } from '../lib/format'
import { useRefresh } from '../lib/refresh'

export default function OrderDetailPage({ id }: { id: number }) {
  const { bump } = useRefresh()
  const [order, setOrder] = useState<Order | null>(null)
  const [loadError, setLoadError] = useState<ApiError | null>(null)
  const [retry, setRetry] = useState(0)
  const [cancelling, setCancelling] = useState(false)
  const [banner, setBanner] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null)

  useEffect(() => {
    const ctrl = new AbortController()
    api
      .getOrder(id, ctrl.signal)
      .then((o) => {
        setOrder(o)
        setLoadError(null)
      })
      .catch((err: unknown) => {
        if (!isAbort(err)) setLoadError(toApiError(err))
      })
    return () => ctrl.abort()
  }, [id, retry])

  async function cancel(o: Order) {
    if (!window.confirm(`Cancel order #${o.id} for ${o.customer}? Its stock will be returned.`)) return
    setCancelling(true)
    setBanner(null)
    try {
      setOrder(await api.cancelOrder(o.id))
      setBanner({ kind: 'ok', text: 'Order cancelled. Its stock has been returned to inventory.' })
      bump()
    } catch (err) {
      const e = toApiError(err)
      setBanner({ kind: 'error', text: e.message })
      if (e.code === 'ALREADY_CANCELLED') setRetry((r) => r + 1) // show the current state
    } finally {
      setCancelling(false)
    }
  }

  if (loadError) {
    return (
      <section>
        <h1>Order #{id}</h1>
        {loadError.code === 'NOT_FOUND' ? (
          <div className="error-state" role="alert">
            <p className="error-title">Order #{id} was not found.</p>
            <p>{loadError.message}</p>
            <a href="#/products">Back to products</a>
          </div>
        ) : (
          <ErrorState
            title={`Could not load order #${id}`}
            error={loadError}
            onRetry={() => {
              setLoadError(null)
              setRetry((r) => r + 1)
            }}
          />
        )}
      </section>
    )
  }

  if (!order) {
    return (
      <section>
        <h1>Order #{id}</h1>
        <p className="muted" aria-live="polite">
          Loading order…
        </p>
      </section>
    )
  }

  const cancelled = order.status === 'CANCELLED'

  return (
    <section aria-labelledby="order-title">
      <div className="page-head">
        <h1 id="order-title">Order #{order.id}</h1>
        {!cancelled && (
          <button type="button" className="btn btn-danger" onClick={() => void cancel(order)} disabled={cancelling}>
            {cancelling ? 'Cancelling…' : 'Cancel order'}
          </button>
        )}
      </div>

      {banner && (
        <div className={`banner banner-${banner.kind}`} role={banner.kind === 'error' ? 'alert' : 'status'}>
          <span>{banner.text}</span>
          {banner.kind === 'ok' && <a href="#/products">View products</a>}
        </div>
      )}

      <dl className="meta">
        <div>
          <dt>Customer</dt>
          <dd>{order.customer}</dd>
        </div>
        <div>
          <dt>Status</dt>
          <dd>
            <span className={`badge ${cancelled ? 'badge-muted' : 'badge-ok'}`}>
              {cancelled ? 'Cancelled' : 'Created'}
            </span>
          </dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd>
            <time dateTime={order.created_at}>{formatDateTime(order.created_at)}</time>
          </dd>
        </div>
        {order.cancelled_at && (
          <div>
            <dt>Cancelled</dt>
            <dd>
              <time dateTime={order.cancelled_at}>{formatDateTime(order.cancelled_at)}</time>
            </dd>
          </div>
        )}
      </dl>

      <div className="table-wrap">
        <table>
          <caption className="sr-only">Items in order #{order.id}</caption>
          <thead>
            <tr>
              <th scope="col">SKU</th>
              <th scope="col" className="num">
                Qty
              </th>
              <th scope="col" className="num">
                Unit price
              </th>
              <th scope="col" className="num">
                Subtotal
              </th>
            </tr>
          </thead>
          <tbody>
            {order.items.map((it) => (
              <tr key={it.sku}>
                <td className="mono">{it.sku}</td>
                <td className="num">{it.qty}</td>
                <td className="num">{formatRupiah(it.unit_price)}</td>
                <td className="num">{formatRupiah(it.subtotal)}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <th scope="row" colSpan={3} className="num">
                Total
              </th>
              <td className="num total">{formatRupiah(order.total)}</td>
            </tr>
          </tfoot>
        </table>
      </div>
    </section>
  )
}
