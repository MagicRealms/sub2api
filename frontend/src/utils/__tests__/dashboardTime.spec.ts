import { describe, expect, it } from 'vitest'
import { dashboardDate, dateInTimeZone } from '../dashboardTime'

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
