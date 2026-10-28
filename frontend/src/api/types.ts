
export interface Product {
  id: number
  sku: string
  name: string
  price: number
  stock: number
  is_active: boolean
  created_at: string
  last_movement_at: string | null
}

export interface Paged<T> {
  data: T[]
  page: number
  limit: number
  total: number
}

export type ProductStatusFilter = '' | 'active' | 'inactive' | 'low'

export interface ProductQuery {
  search?: string
  status?: ProductStatusFilter
  page?: number
  limit?: number
}

export interface ProductInput {
  sku: string
  name: string
  price: number
  is_active?: boolean
}

export interface ProductUpdate {
  name: string
  price: number
  is_active: boolean
}

export type MovementType = 'in' | 'out'

export interface StockInput {
  type: MovementType
  qty: number
  note?: string
}

export interface Movement {
  id: number
  product_id: number
  type: MovementType
  qty: number
  note: string | null
  created_at: string
}

export interface OrderItemInput {
  sku: string
  qty: number
}

export interface OrderInput {
  customer: string
  items: OrderItemInput[]
}

export type OrderStatus = 'CREATED' | 'CANCELLED'

export interface OrderItem {
  sku: string
  qty: number
  unit_price: number
  subtotal: number
}

export interface Order {
  id: number
  customer: string
  status: OrderStatus
  total: number
  created_at: string
  cancelled_at: string | null
  items: OrderItem[]
}

export interface Summary {
  total_products: number
  active_products: number
  total_stock_value: number
  low_stock_count: number
}

export interface FieldErrorDetail {
  field: string
  message: string
}

export interface ErrorEnvelope {
  error: {
    code: string
    message: string
    details?: FieldErrorDetail[]
    sku?: string
    available?: number
  }
}
