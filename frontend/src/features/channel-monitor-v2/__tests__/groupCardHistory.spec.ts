import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorMatrixBucket } from '@/api/channelMonitorV2'
import { groupCardHistory } from '../groupCardHistory'

const coverage: MonitorCoverage = { requested_start: '2026-11-08T00:00:00Z', requested_end: '2026-11-08T01:30:00Z', coverage_start: '2026-11-08T00:00:00Z', data_through: '2026-11-08T01:30:00Z', computed_at: '', coverage_complete: true, aggregation_lag_seconds: 0, bucket_seconds: 60 }
function bucket(minute: number, overall: number | null, cache: number | null): MonitorMatrixBucket {
  return { bucket_start: new Date(Date.parse(coverage.requested_start) + minute*60000).toISOString(), metrics: { ttft: { p50_ms: minute*100 } }, health: { score: overall, cache_score: cache } } as MonitorMatrixBucket
}
describe('V2 group card history', () => {
  it('keeps sparse coverage and selects the worst native sample without averaging P50', () => {
    const a = bucket(0,90,10), b = bucket(1,40,90), future = bucket(90,0,0)
    const slots = groupCardHistory([a,b,future], coverage, 'overall')
    expect(slots).toHaveLength(30)
    expect(slots[0]?.bucket).toBe(b)
    expect(slots[0]?.bucket?.metrics.ttft.p50_ms).toBe(100)
    expect(slots[1]?.bucket).toBeUndefined()
    expect(slots.at(-1)?.end).toBe(Date.parse(coverage.requested_end!))
    expect(groupCardHistory([a,b],coverage,'cache')[0]?.bucket).toBe(a)
  })
  it('does not invent healthy data for missing samples or insufficient coverage', () => {
    const a = bucket(0,null,null)
    expect(groupCardHistory([a],coverage,'overall')[0]?.bucket).toBe(a)
    expect(groupCardHistory([],coverage,'overall').every(slot=>!slot.bucket)).toBe(true)
    expect(groupCardHistory([],{...coverage,requested_start:'invalid'},'overall')).toEqual([])
  })
  it('keeps native buckets for short ranges and bounds long-window rendering', () => {
    const short = {...coverage,bucket_seconds:300}
    expect(groupCardHistory([],short,'overall')).toHaveLength(18)
    const long = {...coverage,requested_end:'2026-12-08T00:00:00Z'}
    expect(groupCardHistory([],long,'overall').length).toBeLessThanOrEqual(36)
  })
  it('retains a red observed state ahead of a lower-scored yellow bucket', () => {
    const red = bucket(0, 90, 90), yellow = bucket(1, 50, 50)
    red.health.overall = 'critical'; yellow.health.overall = 'warning'
    expect(groupCardHistory([yellow, red], coverage, 'overall')[0]?.bucket).toBe(red)
    expect(groupCardHistory([red, yellow], coverage, 'overall')[0]?.bucket).toBe(red)
  })
  it('does not let an unknown bucket hide a real sampled state', () => {
    const empty = bucket(0, null, null), green = bucket(1, 100, 100)
    empty.health.overall = 'unknown'; green.health.overall = 'healthy'
    expect(groupCardHistory([empty, green], coverage, 'overall')[0]?.bucket).toBe(green)
  })

})
