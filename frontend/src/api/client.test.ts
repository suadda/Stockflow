import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, parseApiError } from './client'

describe('parseApiError', () => {
  it('copies code, message and extras from an envelope', () => {
    const body = JSON.stringify({
      error: { code: 'INSUFFICIENT_STOCK', message: 'only 3', sku: 'USB-C-1M', available: 3 },
    })
    const e = parseApiError(409, body)
    expect(e).toBeInstanceOf(ApiError)
    expect(e).toMatchObject({ status: 409, code: 'INSUFFICIENT_STOCK', message: 'only 3', sku: 'USB-C-1M', available: 3 })
  })

  it('keeps validation details and drops malformed ones', () => {
    const body = JSON.stringify({
      error: {
        code: 'VALIDATION_ERROR',
        message: 'validation failed',
        details: [{ field: 'items[0].qty', message: 'bad' }, { nope: 1 }],
      },
    })
    expect(parseApiError(400, body).details).toEqual([{ field: 'items[0].qty', message: 'bad' }])
  })

  it('maps a non-JSON body to UNKNOWN with the status', () => {
    const e = parseApiError(502, '<html>Bad Gateway</html>')
    expect(e).toMatchObject({ status: 502, code: 'UNKNOWN' })
    expect(e.message).toContain('502')
  })

  it.each(['{}', '{"error":"x"}', '{"error":{"message":"m"}}', '[]', 'null', ''])('maps %s to UNKNOWN', (body) => {
    expect(parseApiError(500, body).code).toBe('UNKNOWN')
  })
})

describe('api requests', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('turns a fetch rejection into NETWORK_ERROR with status 0', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(api.getSummary()).rejects.toMatchObject({ status: 0, code: 'NETWORK_ERROR' })
  })

  it('rethrows aborts untouched', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new DOMException('aborted', 'AbortError')))
    await expect(api.getSummary()).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('resolves 204 to undefined', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    await expect(api.deleteProduct(1)).resolves.toBeUndefined()
  })

  it('throws the parsed envelope on non-2xx', async () => {
    const body = JSON.stringify({ error: { code: 'PRODUCT_HAS_HISTORY', message: 'has history' } })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 409 })))
    await expect(api.deleteProduct(1)).rejects.toMatchObject({ status: 409, code: 'PRODUCT_HAS_HISTORY' })
  })

  it('builds query strings without empty values and sends JSON bodies', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ data: [], page: 1, limit: 20, total: 0 }))
    vi.stubGlobal('fetch', fetchMock)
    await api.listProducts({ search: 'usb c', status: '', page: 2 })
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/products?search=usb+c&page=2')

    fetchMock.mockResolvedValue(Response.json({}, { status: 201 }))
    await api.createOrder({ customer: 'A', items: [{ sku: 'X', qty: 1 }] })
    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(url).toBe('/api/orders')
    expect(init.method).toBe('POST')
    expect(JSON.parse(String(init.body))).toEqual({ customer: 'A', items: [{ sku: 'X', qty: 1 }] })
  })
})
