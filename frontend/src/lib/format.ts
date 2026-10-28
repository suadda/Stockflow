export function formatRupiah(n: number): string {
  const digits = Math.trunc(Math.abs(n)).toString()
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${n < 0 ? '-' : ''}Rp ${grouped}`
}

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

export function formatRelative(iso: string | null, now: Date = new Date()): string {
  if (iso === null) return '—'
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return '—'
  const diff = now.getTime() - t
  if (diff < MINUTE) return 'just now'
  if (diff < HOUR) return `${Math.floor(diff / MINUTE)}m ago`
  if (diff < DAY) return `${Math.floor(diff / HOUR)}h ago`
  return `${Math.floor(diff / DAY)}d ago`
}

export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}
