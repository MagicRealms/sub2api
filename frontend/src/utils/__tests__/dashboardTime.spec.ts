import { describe, expect, it, vi } from 'vitest'
import { dashboardDate, dateInTimeZone, reportingDateTimeInput, parseReportingDateTimeInput, reportingTimestamp } from '../dashboardTime'

describe('Beijing reporting dates', () => {
  it.each([
    ['2026-11-08T15:59:59.999Z', '2026-11-08'],
    ['2026-11-08T16:00:00.000Z', '2026-11-09'],
    ['2026-03-08T16:00:00Z', '2026-03-09'],
    ['2026-11-01T16:00:00Z', '2026-11-02']
  ])('cuts over at Beijing midnight for %s', (instant, day) => {
    expect(dashboardDate(new Date(instant))).toBe(day)
    expect(dateInTimeZone(new Date(instant), 'Asia/Shanghai')).toBe(day)
  })
  it('uses calendar arithmetic across the month boundary', () => {
    expect(dashboardDate(new Date('2026-11-30T16:00:00Z'), -6)).toBe('2026-11-25')
  })
})


describe('Beijing reporting datetime inputs', () => {
  it.each(['UTC', 'America/Los_Angeles', 'Asia/Shanghai'])( 'round trips across Beijing midnight (%s)', zone => {
    vi.stubEnv('TZ', zone)
    const at = new Date('2026-11-09T16:05:00Z')
    expect(reportingDateTimeInput(at)).toBe('2026-11-10T00:05')
    expect(parseReportingDateTimeInput('2026-11-10T00:05')?.toISOString()).toBe(at.toISOString())
    vi.unstubAllEnvs()
  })
  it.each(['', 'bad', '2026-02-30T12:00', '2026-11-10T24:00', '2026-11-10T00:05Z'])( 'rejects malformed calendar input %s', value => {
    expect(parseReportingDateTimeInput(value)).toBeNull()
  })
  it('is unaffected by US daylight saving gaps', () => {
    expect(parseReportingDateTimeInput('2026-03-08T02:30')?.toISOString()).toBe('2026-03-07T18:30:00.000Z')
  })
})


it('exports Beijing timestamps with an explicit offset and preserves the instant', () => {
  const value = '2026-11-09T16:05:03Z'
  const result = reportingTimestamp(value)
  expect(result).toBe('2026-11-10T00:05:03.000+08:00')
  expect(new Date(result).toISOString()).toBe(new Date(value).toISOString())
})
