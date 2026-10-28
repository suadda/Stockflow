import { useEffect, useState } from 'react'

export type Route = { name: 'products' } | { name: 'orderNew' } | { name: 'order'; id: number }

/** Maps a location hash to a route; anything unknown shows products. */
export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#/, '')
  if (path === '/orders/new') return { name: 'orderNew' }
  const m = /^\/orders\/(\d+)$/.exec(path)
  if (m) return { name: 'order', id: Number(m[1]) }
  return { name: 'products' }
}

export function navigate(hash: string): void {
  window.location.hash = hash
}

export function useHashRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash))
  useEffect(() => {
    const onChange = () => setRoute(parseRoute(window.location.hash))
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])
  return route
}
