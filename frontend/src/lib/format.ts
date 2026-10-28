/** Formats integer IDR as "Rp 1.234.567". Grouping is manual so the output
 * does not depend on the runtime's locale data. */
export function formatRupiah(n: number): string {
  const digits = Math.trunc(Math.abs(n)).toString()
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${n < 0 ? '-' : ''}Rp ${grouped}`
}

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/** Relative time: "just now", "5m ago", "2h ago", "3d ago"; "—" for null.
 * Future timestamps (clock skew) count as "just now". */
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

/** Absolute local date and time for detail views. */
export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}
