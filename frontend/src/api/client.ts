// The only module that calls fetch or builds API URLs. Every non-2xx
// response becomes a typed ApiError, so screens never parse raw bodies.
import type {
  FieldErrorDetail,
  Movement,
  Order,
  OrderInput,
  Paged,
  Product,
  ProductInput,
  ProductQuery,
  ProductUpdate,
  StockInput,
  Summary,
} from './types'

export interface ApiErrorExtras {
  details?: FieldErrorDetail[]
  sku?: string
  available?: number
}

export class ApiError extends Error {
  /** HTTP status; 0 when no response arrived. */
  readonly status: number
  /** Envelope code, or 'UNKNOWN' / 'NETWORK_ERROR'. */
  readonly code: string
  readonly details?: FieldErrorDetail[]
  readonly sku?: string
  readonly available?: number

  constructor(status: number, code: string, message: string, extras: ApiErrorExtras = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    if (extras.details !== undefined) this.details = extras.details
    if (extras.sku !== undefined) this.sku = extras.sku
    if (extras.available !== undefined) this.available = extras.available
  }
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function isDetail(v: unknown): v is FieldErrorDetail {
  return isRecord(v) && typeof v.field === 'string' && typeof v.message === 'string'
}

/**
 * Turns a non-2xx response body into an ApiError. Pure and total: it never
 * throws. Bodies that are not an error envelope become code UNKNOWN.
 */
export function parseApiError(status: number, bodyText: string): ApiError {
  let parsed: unknown
  try {
    parsed = JSON.parse(bodyText)
  } catch {
    parsed = undefined
  }
  const e = isRecord(parsed) ? parsed.error : undefined
  if (!isRecord(e) || typeof e.code !== 'string' || e.code === '') {
    return new ApiError(status, 'UNKNOWN', `Request failed (HTTP ${status})`)
  }
  const extras: ApiErrorExtras = {}
  if (Array.isArray(e.details)) extras.details = e.details.filter(isDetail)
  if (typeof e.sku === 'string') extras.sku = e.sku
  if (typeof e.available === 'number') extras.available = e.available
  const message = typeof e.message === 'string' && e.message !== '' ? e.message : e.code
  return new ApiError(status, e.code, message, extras)
}

/** Normalises anything thrown by an api call into an ApiError. */
export function toApiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err
  return new ApiError(0, 'UNKNOWN', err instanceof Error ? err.message : String(err))
}

/** True for the error fetch throws when a request is aborted. */
export function isAbort(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const init: RequestInit = { method }
  if (signal) init.signal = signal
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }

  let res: Response
  try {
    res = await fetch(path, init)
  } catch (err) {
    if (isAbort(err)) throw err
    throw new ApiError(0, 'NETWORK_ERROR', 'Network error: could not reach the server')
  }
  if (!res.ok) throw parseApiError(res.status, await res.text())
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

function withQuery(path: string, q: Record<string, string | number | undefined>): string {
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(q)) {
    if (v !== undefined && v !== '') params.set(k, String(v))
  }
  const qs = params.toString()
  return qs ? `${path}?${qs}` : path
}

export const api = {
  listProducts: (q: ProductQuery, signal?: AbortSignal) =>
    request<Paged<Product>>('GET', withQuery('/api/products', { ...q }), undefined, signal),
  createProduct: (input: ProductInput) => request<Product>('POST', '/api/products', input),
  updateProduct: (id: number, input: ProductUpdate) => request<Product>('PUT', `/api/products/${id}`, input),
  deleteProduct: (id: number) => request<void>('DELETE', `/api/products/${id}`),
  adjustStock: (id: number, input: StockInput) => request<Product>('POST', `/api/products/${id}/stock`, input),
  listMovements: (id: number) => request<{ data: Movement[] }>('GET', `/api/products/${id}/movements`),
  createOrder: (input: OrderInput) => request<Order>('POST', '/api/orders', input),
  getOrder: (id: number, signal?: AbortSignal) => request<Order>('GET', `/api/orders/${id}`, undefined, signal),
  cancelOrder: (id: number) => request<Order>('POST', `/api/orders/${id}/cancel`),
  getSummary: (signal?: AbortSignal) => request<Summary>('GET', '/api/summary', undefined, signal),
}
