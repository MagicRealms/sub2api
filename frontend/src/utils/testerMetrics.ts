export type TesterPeriod = 'today' | 'week' | 'month' | 'session'

// Calendar days are anchored to Beijing, independent of the browser/server TZ.
export function testerRange(period: TesterPeriod, now = new Date(), visibleAfter?: string | null) {
  const day = new Date(now.getTime() + 8 * 3600000).toISOString().slice(0, 10)
  const midnight = new Date(`${day}T00:00:00+08:00`).getTime()
  const days = period === 'week' ? 6 : period === 'month' ? 29 : 0
  let start = midnight - days * 86400000
  if (period === 'session') {
    const cleared = visibleAfter ? Date.parse(visibleAfter) : NaN
    start = Number.isFinite(cleared) ? Math.max(cleared, midnight - 29 * 86400000) : midnight
  }
  start = Math.min(start, now.getTime())
  return { start_time: new Date(start).toISOString(), end_time: new Date(Math.max(now.getTime(), start + 1)).toISOString(), timezone: 'Asia/Shanghai' }
}

export function testerCacheRate(row: { input_tokens: number; cache_creation_tokens: number; cache_read_tokens: number }): number {
  const total = row.input_tokens + row.cache_creation_tokens + row.cache_read_tokens
  return total > 0 ? Math.max(0, Math.min(100, row.cache_read_tokens * 100 / total)) : 0
}

export function testerTime(ms?: number | null): string {
  return ms != null && Number.isFinite(ms) ? `${(ms / 1000).toFixed(2)} s` : '—'
}

export function testerMoney(amount?: number | null): string {
  return amount != null && Number.isFinite(amount) ? `$${amount.toFixed(amount > 0 && amount < 0.000001 ? 8 : 6)}` : '—'
}

export function testerTokens(tokens: number): string {
  if (tokens > 0 && tokens < 100) return '<0.0001 M'
  return `${(tokens / 1000000).toFixed(tokens > 0 && tokens < 10000 ? 4 : 2)} M`
}
