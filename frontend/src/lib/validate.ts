import type { FieldErrorDetail, OrderInput } from '../api/types'

export const MAX_QTY = 1_000_000
export const MAX_PRICE = 2_147_483_647

export function validateQty(raw: string): string | null {
  if (!/^\d+$/.test(raw)) return 'Enter a whole number from 1 to 1,000,000'
  const n = Number(raw)
  if (n < 1 || n > MAX_QTY) return 'Enter a whole number from 1 to 1,000,000'
  return null
}

export interface ProductFormValues {
  sku: string
  name: string
  price: string
}

export type ProductFormErrors = Partial<Record<keyof ProductFormValues, string>>

export function validateProductForm(f: ProductFormValues): ProductFormErrors {
  const errs: ProductFormErrors = {}
  if (f.sku.trim() === '') errs.sku = 'SKU is required'
  if (f.name.trim() === '') errs.name = 'Name is required'
  if (!/^\d+$/.test(f.price)) errs.price = 'Price must be a whole number of rupiah'
  else if (Number(f.price) > MAX_PRICE) errs.price = 'Price is too large'
  return errs
}

export interface OrderRow {
  sku: string
  qty: string
}

export interface OrderDraft {
  customer: string
  rows: OrderRow[]
}

export interface OrderDraftErrors {
  customer?: string
  rows: Record<number, { sku?: string; qty?: string }>
  form?: string
}

export function skuKey(sku: string): string {
  return sku.trim().toUpperCase()
}

export function validateOrderDraft(d: OrderDraft): OrderDraftErrors {
  const errs: OrderDraftErrors = { rows: {} }
  if (d.customer.trim() === '') errs.customer = 'Customer is required'
  if (d.rows.length === 0) errs.form = 'Add at least one item'

  const seen = new Set<string>()
  d.rows.forEach((row, i) => {
    const e: { sku?: string; qty?: string } = {}
    const key = skuKey(row.sku)
    if (key === '') e.sku = 'SKU is required'
    else if (seen.has(key)) e.sku = 'Duplicate SKU'
    else seen.add(key)
    const q = validateQty(row.qty)
    if (q !== null) e.qty = q
    if (e.sku !== undefined || e.qty !== undefined) errs.rows[i] = e
  })
  return errs
}

export function hasDraftErrors(e: OrderDraftErrors): boolean {
  return e.customer !== undefined || e.form !== undefined || Object.keys(e.rows).length > 0
}

const KNOWN_FIELD = /^(customer|items|items\[\d+\]\.(sku|qty))$/

export function mapValidationDetails(details: FieldErrorDetail[]): {
  fields: Record<string, string[]>
  unmatched: string[]
} {
  const fields: Record<string, string[]> = {}
  const unmatched: string[] = []
  for (const d of details) {
    if (KNOWN_FIELD.test(d.field)) (fields[d.field] ??= []).push(d.message)
    else unmatched.push(d.field ? `${d.field}: ${d.message}` : d.message)
  }
  return { fields, unmatched }
}

export function toOrderRequest(d: OrderDraft): OrderInput {
  return {
    customer: d.customer.trim(),
    items: d.rows.map((r) => ({ sku: r.sku.trim(), qty: Number(r.qty) })),
  }
}
