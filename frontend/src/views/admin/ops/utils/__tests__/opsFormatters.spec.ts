import { describe, expect, it } from 'vitest'

import { formatMemorySizeMB, formatDateTime, formatHistoryLabel } from '../opsFormatters'

describe('formatMemorySizeMB', () => {
  it('keeps capacities below one GiB in MB', () => {
    expect(formatMemorySizeMB(61)).toBe('61 MB')
    expect(formatMemorySizeMB(1023)).toBe('1023 MB')
  })

  it('converts capacities at or above one GiB to GB', () => {
    expect(formatMemorySizeMB(1024)).toBe('1 GB')
    expect(formatMemorySizeMB(1536)).toBe('1.5 GB')
    expect(formatMemorySizeMB(32768)).toBe('32 GB')
  })

  it('handles zero and invalid values without producing a misleading unit', () => {
    expect(formatMemorySizeMB(0)).toBe('0 MB')
    expect(formatMemorySizeMB(-1)).toBe('-')
    expect(formatMemorySizeMB(Number.NaN)).toBe('-')
    expect(formatMemorySizeMB(Number.POSITIVE_INFINITY)).toBe('-')
    expect(formatMemorySizeMB(null)).toBe('-')
    expect(formatMemorySizeMB(undefined)).toBe('-')
  })
})


describe('ops Beijing reporting labels', () => {
  it('uses the next Beijing calendar day after 16:00 UTC', () => {
    expect(formatDateTime('2026-11-09T16:05:03Z')).toBe('11-10 00:05:03')
    expect(formatHistoryLabel('2026-11-09T16:05:03Z', '24h')).toBe('11-10 00:05')
    expect(formatHistoryLabel('2026-11-09T16:05:03Z', '1h')).toBe('00:05')
  })
})
