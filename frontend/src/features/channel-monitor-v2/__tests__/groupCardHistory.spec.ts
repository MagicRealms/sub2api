import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorMatrixBucket, MonitorProbeObservation } from '@/api/channelMonitorV2'
import { groupCardHistory, groupHistoryState } from '../groupCardHistory'

const coverage = { requested_start: '2026-11-08T00:00:00Z', requested_end: '2026-11-08T01:30:00Z', bucket_seconds: 300 } as MonitorCoverage
function bucket(minute: number, state = 'healthy'): MonitorMatrixBucket {
  return { bucket_start: new Date(Date.parse(coverage.requested_start) + minute * 60000).toISOString(), metrics: { ttft: { p50_ms: minute * 100 } }, health: { overall: state } } as MonitorMatrixBucket
}

describe('V2 group card observation history', () => {
  it('keeps the latest twelve samples in chronological order without averaging native metrics', () => {
    const samples = Array.from({ length: 20 }, (_, i) => bucket(i))
    const slots = groupCardHistory(samples, coverage, 'overall')
    expect(slots).toHaveLength(12)
    expect(slots[0]!.bucket).toBe(samples[8])
    expect(slots[11]!.bucket).toBe(samples[19])
    expect(slots[0]!.bucket!.metrics.ttft.p50_ms).toBe(800)
  })
  it('preserves old observations when the wall-clock range advances', () => {
    const old = bucket(-120, 'warning')
    const slots = groupCardHistory([old], coverage, 'overall')
    expect(slots).toHaveLength(12)
    expect(slots.every(slot => slot.bucket === old)).toBe(true)
    expect(slots.filter(slot => slot.carried)).toHaveLength(11)
    expect(slots[11]!.carried).toBeUndefined()
  })
  it('renders twelve unknown cells until an observation exists', () => {
    const slots = groupCardHistory([], coverage, 'overall')
    expect(slots).toHaveLength(12)
    expect(slots.every(slot => groupHistoryState(slot, 'overall') === 'unknown')).toBe(true)
  })
  it('mixes probes and real traffic, maps actual failure to red, and retains probe status for idle channels', () => {
    const probes: MonitorProbeObservation[] = [
      { checked_at: bucket(-60).bucket_start, status: 'operational', model: 'test' },
      { checked_at: bucket(40).bucket_start, status: 'degraded', model: 'test' },
      { checked_at: bucket(60).bucket_start, status: 'error', model: 'test' },
    ]
    const slots = groupCardHistory([bucket(0)], coverage, 'overall', undefined, probes)
    expect(slots).toHaveLength(12)
    expect(slots[9]!.bucket).toBeDefined()
    expect(groupHistoryState(slots[10]!, 'overall')).toBe('warning')
    expect(groupHistoryState(slots[11]!, 'overall')).toBe('critical')
    expect(slots[11]!.bucket).toBeUndefined()
  })
  it('deduplicates refreshed native buckets and does not fabricate new probes', () => {
    const sample = bucket(0)
    const slots = groupCardHistory([sample, sample], coverage, 'overall', sample)
    expect(slots.filter(slot => !slot.carried)).toHaveLength(1)
    expect(slots.some(slot => slot.probe)).toBe(false)
  })
})
