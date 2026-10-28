import { useEffect, useState } from 'react'
import { api, type ApiError, isAbort, toApiError } from '../api/client'
import type { Summary } from '../api/types'
import { formatRupiah } from '../lib/format'
import { useRefresh } from '../lib/refresh'
import { ErrorState } from './ErrorState'

export function SummaryCards() {
  const { version } = useRefresh()
  const [summary, setSummary] = useState<Summary | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    const ctrl = new AbortController()
    api
      .getSummary(ctrl.signal)
      .then((s) => {
        setSummary(s)
        setError(null)
      })
      .catch((err: unknown) => {
        if (!isAbort(err)) setError(toApiError(err))
      })
    return () => ctrl.abort()
  }, [version, retry])

  if (error) {
    return (
      <ErrorState
        title="Could not load summary"
        error={error}
        onRetry={() => {
          setError(null)
          setRetry((r) => r + 1)
        }}
      />
    )
  }

  const cards = [
    { label: 'Total products', value: summary?.total_products.toLocaleString('id-ID') },
    { label: 'Active products', value: summary?.active_products.toLocaleString('id-ID') },
    { label: 'Total stock value', value: summary ? formatRupiah(summary.total_stock_value) : undefined },
    { label: 'Low-stock products', value: summary?.low_stock_count.toLocaleString('id-ID') },
  ]

  return (
    <dl className="cards" aria-busy={summary === null}>
      {cards.map((c) => (
        <div className="card" key={c.label}>
          <dt>{c.label}</dt>
          <dd>{c.value ?? '…'}</dd>
        </div>
      ))}
    </dl>
  )
}
