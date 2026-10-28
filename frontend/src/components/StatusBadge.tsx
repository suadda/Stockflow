import type { Product } from '../api/types'

export const LOW_STOCK_THRESHOLD = 10

export function StatusBadge({ product }: { product: Pick<Product, 'is_active' | 'stock'> }) {
  return (
    <span className="badges">
      {product.is_active ? (
        <span className="badge badge-ok">Active</span>
      ) : (
        <span className="badge badge-muted">Inactive</span>
      )}
      {product.stock <= LOW_STOCK_THRESHOLD && <span className="badge badge-warn">Low stock</span>}
    </span>
  )
}
