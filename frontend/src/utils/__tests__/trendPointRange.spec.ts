import { describe, expect, it } from 'vitest'
import { getTrendPointRange } from '../trendPointRange'

describe('getTrendPointRange', () => {
  it('converts hourly labels in the server timezone to exact UTC boundaries', () => {
    const range = getTrendPointRange('2026-11-08 10:00', 'hour', 'America/Los_Angeles')
    expect(range?.start.toISOString()).toBe('2026-11-08T18:00:00.000Z')
    expect(range?.end.toISOString()).toBe('2026-11-08T19:00:00.000Z')
  })

  it('includes both copies of an hour repeated when daylight saving time ends', () => {
    const range = getTrendPointRange('2026-11-01 01:00', 'hour', 'America/Los_Angeles')
    expect(range?.start.toISOString()).toBe('2026-11-01T08:00:00.000Z')
    expect(range?.end.toISOString()).toBe('2026-11-01T10:00:00.000Z')
  })

  it('uses a 23-hour day when daylight saving time starts', () => {
    const range = getTrendPointRange('2026-03-08', 'day', 'America/Los_Angeles')
    expect(range?.start.toISOString()).toBe('2026-03-08T08:00:00.000Z')
    expect(range?.end.toISOString()).toBe('2026-03-09T07:00:00.000Z')
  })
})
