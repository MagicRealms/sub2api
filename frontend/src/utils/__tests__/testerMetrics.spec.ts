import { describe, expect, it } from 'vitest'
import { testerCacheRate, testerMoney, testerRange, testerTime, testerTokens } from '../testerMetrics'

describe('tester observation metrics', () => {
  it('uses Beijing calendar dates at the UTC date boundary', () => {
    const range = testerRange('today', new Date('2026-11-08T16:30:00Z'))
    expect(range.start_time).toBe('2026-11-08T16:00:00.000Z')
    expect(range.timezone).toBe('Asia/Shanghai')
  })
  it('includes seven calendar days and caps a session at thirty days', () => {
    const now = new Date('2026-11-09T10:00:00Z')
    expect(testerRange('week', now).start_time).toBe('2026-11-02T16:00:00.000Z')
    expect(testerRange('session', now, '2020-01-01T00:00:00Z')).toEqual(testerRange('month', now))
  })
  it('starts at the actual reset and leaves missing samples unknown', () => {
    const range = testerRange('session', new Date('2026-11-09T10:00:00Z'), '2026-11-09T09:10:11Z')
    expect(range.start_time).toBe('2026-11-09T09:10:11.000Z')
    expect(testerTime(null)).toBe('—')
    expect(testerMoney(null)).toBe('—')
    expect(testerTime(1000)).toBe('1.00 s')
  })
  it('uses the same cache token denominator as ops, including writes', () => {
    expect(testerCacheRate({ input_tokens:100,cache_read_tokens:900,cache_creation_tokens:1000 })).toBe(45)
    expect(testerCacheRate({ input_tokens:0,cache_read_tokens:0,cache_creation_tokens:0 })).toBe(0)
  })
  it('keeps midnight in the current Beijing day and shows small positive measurements', () => {
    const range = testerRange('today', new Date('2026-11-08T16:00:00Z'))
    expect(range.start_time).toBe('2026-11-08T16:00:00.000Z')
    expect(range.end_time).toBe('2026-11-08T16:00:00.001Z')
    expect(testerMoney(0.00000023)).toBe('$0.00000023')
    expect(testerTokens(12)).toBe('<0.0001 M')
    expect(testerTokens(0)).toBe('0.00 M')
  })
})
