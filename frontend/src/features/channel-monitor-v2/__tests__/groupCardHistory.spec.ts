import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorMatrixBucket } from '@/api/channelMonitorV2'
import { groupCardHistory } from '../groupCardHistory'

const coverage: MonitorCoverage = { requested_start: '2026-11-08T00:00:00Z', requested_end: '2026-11-08T01:30:00Z', coverage_start: '2026-11-08T00:00:00Z', data_through: '2026-11-08T01:30:00Z', computed_at: '', coverage_complete: true, aggregation_lag_seconds: 0, bucket_seconds: 60 }
function bucket(minute: number, overall: number | null, cache: number | null): MonitorMatrixBucket {
  return { bucket_start: new Date(Date.parse(coverage.requested_start) + minute*60000).toISOString(), metrics: { ttft: { p50_ms: minute*100 } }, health: { score: overall, cache_score: cache } } as MonitorMatrixBucket
}
describe('V2 group card history', () => {
  it('skips idle intervals and selects the worst native sample without averaging P50', () => {
    const a = bucket(0,90,10), b = bucket(1,40,90), future = bucket(90,0,0)
    const slots = groupCardHistory([a,b,future], coverage, 'overall')
    expect(slots).toHaveLength(1)
    expect(slots[0]?.bucket).toBe(b)
    expect(slots[0]?.bucket?.metrics.ttft.p50_ms).toBe(100)
    expect(groupCardHistory([a,b],coverage,'cache')[0]?.bucket).toBe(a)
  })
  it('keeps observed unknown buckets but omits intervals with no samples', () => {
    const a = bucket(0,null,null)
    expect(groupCardHistory([a],coverage,'overall')[0]?.bucket).toBe(a)
    expect(groupCardHistory([],coverage,'overall')).toEqual([])
    expect(groupCardHistory([],{...coverage,requested_start:'invalid'},'overall')).toEqual([])
  })
  it('bounds aggregation density for short and long ranges', () => {
    const short = {...coverage,bucket_seconds:300}
    const long = {...coverage,requested_end:'2026-12-08T00:00:00Z'}
    const samples = Array.from({ length: 50 }, (_, minute) => bucket(minute, 80, 80))
    expect(groupCardHistory(samples,short,'overall').length).toBeLessThanOrEqual(18)
    expect(groupCardHistory(samples,long,'overall').length).toBeLessThanOrEqual(36)
  })
  it('preserves sample order while collapsing long idle gaps', () => {
    const samples = [bucket(0,90,90), bucket(8,80,80), bucket(89,70,70)]
    const slots = groupCardHistory(samples,coverage,'overall')
    expect(slots).toHaveLength(3)
    expect(slots.map((slot) => slot.bucket?.bucket_start)).toEqual(samples.map((sample) => sample.bucket_start))
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
