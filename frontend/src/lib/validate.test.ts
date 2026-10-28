import { describe, expect, it } from 'vitest'
import {
  hasDraftErrors,
  mapValidationDetails,
  toOrderRequest,
  validateOrderDraft,
  validateProductForm,
  validateQty,
} from './validate'

describe('validateQty', () => {
  it.each(['1', '42', '1000000'])('accepts %s', (s) => {
    expect(validateQty(s)).toBeNull()
  })
  it.each(['', '0', '-3', '1.5', ' 2', '2 ', '1e3', 'abc', '1000001', '00'])('rejects %j', (s) => {
    expect(validateQty(s)).not.toBeNull()
  })
})

describe('validateProductForm', () => {
  it('accepts a valid product', () => {
    expect(validateProductForm({ sku: 'A-1', name: 'Cable', price: '25000' })).toEqual({})
  })
  it('reports blank fields and bad prices', () => {
    expect(validateProductForm({ sku: ' ', name: '', price: '-1' })).toEqual({
      sku: expect.any(String),
      name: expect.any(String),
      price: expect.any(String),
    })
    expect(validateProductForm({ sku: 'A', name: 'B', price: '1.5' }).price).toBeDefined()
    expect(validateProductForm({ sku: 'A', name: 'B', price: '2147483648' }).price).toBeDefined()
    expect(validateProductForm({ sku: 'A', name: 'B', price: '0' })).toEqual({})
  })
})

describe('validateOrderDraft', () => {
  it('accepts a complete draft', () => {
    const e = validateOrderDraft({ customer: 'Toko Maju', rows: [{ sku: 'A', qty: '2' }] })
    expect(hasDraftErrors(e)).toBe(false)
  })
  it('reports blank customer and no rows', () => {
    const e = validateOrderDraft({ customer: '  ', rows: [] })
    expect(e.customer).toBeDefined()
    expect(e.form).toBeDefined()
    expect(hasDraftErrors(e)).toBe(true)
  })
  it('flags blank sku, bad qty and duplicates ignoring case and spaces', () => {
    const e = validateOrderDraft({
      customer: 'X',
      rows: [
        { sku: 'usb-c-1m', qty: '1' },
        { sku: '', qty: '0' },
        { sku: ' USB-C-1M ', qty: '2' },
      ],
    })
    expect(e.rows[0]).toBeUndefined()
    expect(e.rows[1]).toEqual({ sku: expect.any(String), qty: expect.any(String) })
    expect(e.rows[2]?.sku).toBe('Duplicate SKU')
  })
})

describe('mapValidationDetails', () => {
  it('routes known fields and keeps every other message', () => {
    const { fields, unmatched } = mapValidationDetails([
      { field: 'customer', message: 'customer is required' },
      { field: 'items[1].sku', message: 'unknown sku' },
      { field: 'items[1].sku', message: 'second message' },
      { field: 'items', message: 'too big' },
      { field: 'note', message: 'odd' },
    ])
    expect(fields).toEqual({
      customer: ['customer is required'],
      'items[1].sku': ['unknown sku', 'second message'],
      items: ['too big'],
    })
    expect(unmatched).toEqual(['note: odd'])
  })
})

describe('toOrderRequest', () => {
  it('trims and sends only customer and items', () => {
    expect(toOrderRequest({ customer: ' Toko ', rows: [{ sku: ' A-1 ', qty: '3' }] })).toEqual({
      customer: 'Toko',
      items: [{ sku: 'A-1', qty: 3 }],
    })
  })
})
