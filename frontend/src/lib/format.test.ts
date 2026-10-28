import { describe, expect, it } from 'vitest'
import { formatRelative, formatRupiah } from './format'

describe('formatRupiah', () => {
  it.each([
    [0, 'Rp 0'],
    [999, 'Rp 999'],
    [1000, 'Rp 1.000'],
    [25000, 'Rp 25.000'],
    [1234567, 'Rp 1.234.567'],
    [2147483647000000, 'Rp 2.147.483.647.000.000'],
  ])('%d -> %s', (n, want) => {
    expect(formatRupiah(n)).toBe(want)
  })
})

describe('formatRelative', () => {
  const now = new Date('2026-10-28T12:00:00Z')
  const ago = (ms: number) => new Date(now.getTime() - ms).toISOString()
  const MIN = 60_000

  it.each([
    [ago(0), 'just now'],
    [ago(59_999), 'just now'],
    [ago(MIN), '1m ago'],
    [ago(5 * MIN), '5m ago'],
    [ago(59 * MIN + 59_999), '59m ago'],
    [ago(2 * 60 * MIN), '2h ago'],
    [ago(24 * 60 * MIN - 1), '23h ago'],
    [ago(3 * 24 * 60 * MIN), '3d ago'],
    [ago(-10 * MIN), 'just now'],
  ])('%s -> %s', (iso, want) => {
    expect(formatRelative(iso, now)).toBe(want)
  })

  it('shows a dash for null or garbage', () => {
    expect(formatRelative(null, now)).toBe('—')
    expect(formatRelative('not a date', now)).toBe('—')
  })
})
