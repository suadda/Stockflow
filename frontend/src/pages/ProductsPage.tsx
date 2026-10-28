import { useEffect, useState } from 'react'
import { api, ApiError, isAbort, toApiError } from '../api/client'
import type { Paged, Product, ProductStatusFilter } from '../api/types'
import { ErrorState } from '../components/ErrorState'
import { ProductFormModal } from '../components/ProductFormModal'
import { StatusBadge } from '../components/StatusBadge'
import { formatDateTime, formatRelative, formatRupiah } from '../lib/format'
import { useDebounce } from '../lib/useDebounce'
import { useRefresh } from '../lib/refresh'

const FILTERS: { value: ProductStatusFilter; label: string }[] = [
  { value: '', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Inactive' },
  { value: 'low', label: 'Low stock' },
]

const LIMIT = 20

type Editing = { mode: 'create' } | { mode: 'edit'; product: Product } | null
type Banner = { kind: 'ok' | 'error'; text: string } | null

export default function ProductsPage() {
  const { version, bump } = useRefresh()

  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounce(search.trim(), 300)
  const [status, setStatus] = useState<ProductStatusFilter>('')

  // The page resets to 1 whenever the search or filter changes: a page
  // number only counts for the filter it was chosen under.
  const filterKey = `${debouncedSearch}\u0000${status}`
  const [paging, setPaging] = useState({ key: filterKey, page: 1 })
  const page = paging.key === filterKey ? paging.page : 1
  const goTo = (p: number) => setPaging({ key: filterKey, page: p })

  const [retry, setRetry] = useState(0)
  const [data, setData] = useState<Paged<Product> | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const [loading, setLoading] = useState(true)

  const [editing, setEditing] = useState<Editing>(null)
  const [banner, setBanner] = useState<Banner>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  useEffect(() => {
    const ctrl = new AbortController()
    setLoading(true)
    api
      .listProducts({ search: debouncedSearch, status, page, limit: LIMIT }, ctrl.signal)
      .then((res) => {
        setData(res)
        setError(null)
      })
      .catch((err: unknown) => {
        if (!isAbort(err)) setError(toApiError(err))
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false)
      })
    return () => ctrl.abort() // drops stale responses when inputs change
  }, [debouncedSearch, status, page, version, retry])

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1

  // Deleting the last row of the last page leaves us past the end.
  useEffect(() => {
    if (data && data.data.length === 0 && data.total > 0 && page > totalPages) {
      setPaging({ key: filterKey, page: totalPages })
    }
  }, [data, page, totalPages, filterKey])

  async function remove(p: Product) {
    if (!window.confirm(`Delete ${p.sku} (${p.name})? This cannot be undone.`)) return
    setDeletingId(p.id)
    setBanner(null)
    try {
      await api.deleteProduct(p.id)
      setBanner({ kind: 'ok', text: `Deleted ${p.sku}.` })
      bump()
    } catch (err) {
      const e = toApiError(err)
      setBanner({
        kind: 'error',
        text:
          e.code === 'PRODUCT_HAS_HISTORY'
            ? `${p.sku} has stock or order history and cannot be deleted. Mark it inactive instead.`
            : e.message,
      })
    } finally {
      setDeletingId(null)
    }
  }

  const clearFilters = () => {
    setSearch('')
    setStatus('')
  }

  let body
  if (error) {
    body = (
      <ErrorState
        title="Could not load products"
        error={error}
        onRetry={() => {
          setError(null)
          setRetry((r) => r + 1)
        }}
      />
    )
  } else if (!data) {
    body = <SkeletonTable />
  } else if (data.total === 0) {
    body = (
      <div className="empty-state">
        <p>No products match these filters.</p>
        {(search !== '' || status !== '') && (
          <button type="button" className="btn" onClick={clearFilters}>
            Clear search and filters
          </button>
        )}
      </div>
    )
  } else {
    body = (
      <>
        <div className="table-wrap">
          <table aria-busy={loading}>
            <caption className="sr-only">
              Products, page {page} of {totalPages}
            </caption>
            <thead>
              <tr>
                <th scope="col">SKU</th>
                <th scope="col">Name</th>
                <th scope="col" className="num">
                  Price
                </th>
                <th scope="col" className="num">
                  Stock
                </th>
                <th scope="col">Status</th>
                <th scope="col">Last movement</th>
                <th scope="col">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {data.data.map((p) => (
                <tr key={p.id}>
                  <td className="mono">{p.sku}</td>
                  <td>{p.name}</td>
                  <td className="num">{formatRupiah(p.price)}</td>
                  <td className="num">{p.stock}</td>
                  <td>
                    <StatusBadge product={p} />
                  </td>
                  <td>
                    {p.last_movement_at ? (
                      <time dateTime={p.last_movement_at} title={formatDateTime(p.last_movement_at)}>
                        {formatRelative(p.last_movement_at)}
                      </time>
                    ) : (
                      <span aria-label="No movements">—</span>
                    )}
                  </td>
                  <td className="actions">
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => setEditing({ mode: 'edit', product: p })}
                      aria-label={`Edit ${p.sku}`}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm btn-danger"
                      onClick={() => void remove(p)}
                      disabled={deletingId === p.id}
                      aria-label={`Delete ${p.sku}`}
                    >
                      {deletingId === p.id ? 'Deleting…' : 'Delete'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {data.total > data.limit && (
          <nav className="pagination" aria-label="Pagination">
            <button type="button" className="btn" onClick={() => goTo(page - 1)} disabled={page <= 1 || loading}>
              Previous
            </button>
            <span>
              Page {page} of {totalPages}
            </span>
            <button
              type="button"
              className="btn"
              onClick={() => goTo(page + 1)}
              disabled={page >= totalPages || loading}
            >
              Next
            </button>
          </nav>
        )}
      </>
    )
  }

  return (
    <section aria-labelledby="products-title">
      <div className="page-head">
        <h1 id="products-title">Products</h1>
        <button type="button" className="btn btn-primary" onClick={() => setEditing({ mode: 'create' })}>
          New product
        </button>
      </div>

      <div className="toolbar">
        <div className="field-inline">
          <label htmlFor="product-search">Search SKU or name</label>
          <input
            id="product-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            autoComplete="off"
          />
        </div>
        <div className="segmented" role="group" aria-label="Filter products">
          {FILTERS.map((f) => (
            <button
              key={f.value || 'all'}
              type="button"
              aria-pressed={status === f.value}
              onClick={() => setStatus(f.value)}
            >
              {f.label}
            </button>
          ))}
        </div>
      </div>

      {banner && (
        <div className={`banner banner-${banner.kind}`} role={banner.kind === 'error' ? 'alert' : 'status'}>
          <span>{banner.text}</span>
          <button type="button" className="link-btn" onClick={() => setBanner(null)}>
            Dismiss
          </button>
        </div>
      )}

      <p className="muted result-count" aria-live="polite">
        {loading ? 'Loading products…' : data && !error ? `${data.total} product${data.total === 1 ? '' : 's'}` : ''}
      </p>

      {body}

      {editing && (
        <ProductFormModal
          product={editing.mode === 'edit' ? editing.product : undefined}
          onClose={() => setEditing(null)}
          onSaved={(p) => {
            setEditing(null)
            setBanner({ kind: 'ok', text: `Saved ${p.sku}.` })
            bump()
          }}
        />
      )}
    </section>
  )
}

function SkeletonTable() {
  return (
    <div className="table-wrap" aria-hidden="true">
      <table className="skeleton">
        <tbody>
          {Array.from({ length: 6 }, (_, i) => (
            <tr key={i}>
              {Array.from({ length: 7 }, (_, j) => (
                <td key={j}>
                  <span className="skeleton-bar" />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
