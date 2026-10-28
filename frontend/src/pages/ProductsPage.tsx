import { useEffect, useState } from 'react'
import { api, ApiError, isAbort } from '../api/client'
import type { Product } from '../api/types'
import { formatRupiah } from '../lib/format'

// Placeholder for task 9: a bare list proving the real API is reachable
// through the Vite proxy. Task 9 replaces it with the full products page.
export default function ProductsPage() {
  const [items, setItems] = useState<Product[] | null>(null)
  const [error, setError] = useState<ApiError | null>(null)

  useEffect(() => {
    const ctrl = new AbortController()
    api
      .listProducts({}, ctrl.signal)
      .then((p) => setItems(p.data))
      .catch((err: unknown) => {
        if (isAbort(err)) return
        setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', String(err)))
      })
    return () => ctrl.abort()
  }, [])

  if (error) return <p role="alert">{error.message}</p>
  if (!items) return <p>Loading…</p>
  return (
    <ul>
      {items.map((p) => (
        <li key={p.id}>
          {p.sku} · {p.name} · {formatRupiah(p.price)} · stock {p.stock}
        </li>
      ))}
    </ul>
  )
}
