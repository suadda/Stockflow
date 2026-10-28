import { useState, type FormEvent } from 'react'
import { navigate, useHashRoute } from './lib/useHashRoute'
import OrderDetailPage from './pages/OrderDetailPage'
import OrderFormPage from './pages/OrderFormPage'
import ProductsPage from './pages/ProductsPage'

export default function App() {
  const route = useHashRoute()
  const [orderId, setOrderId] = useState('')

  const openOrder = (e: FormEvent) => {
    e.preventDefault()
    if (/^\d+$/.test(orderId.trim())) navigate(`#/orders/${orderId.trim()}`)
  }

  return (
    <>
      <header className="app-header">
        <a className="brand" href="#/products">
          StockFlow
        </a>
        <nav aria-label="Main">
          <a href="#/products" aria-current={route.name === 'products' ? 'page' : undefined}>
            Products
          </a>
          <a href="#/orders/new" aria-current={route.name === 'orderNew' ? 'page' : undefined}>
            New order
          </a>
        </nav>
        <form className="open-order" onSubmit={openOrder}>
          <label htmlFor="open-order-id">Open order #</label>
          <input
            id="open-order-id"
            inputMode="numeric"
            value={orderId}
            onChange={(e) => setOrderId(e.target.value)}
            size={6}
          />
          <button type="submit">Open</button>
        </form>
      </header>
      <main>
        {route.name === 'products' && <ProductsPage />}
        {route.name === 'orderNew' && <OrderFormPage />}
        {route.name === 'order' && <OrderDetailPage id={route.id} />}
      </main>
    </>
  )
}
