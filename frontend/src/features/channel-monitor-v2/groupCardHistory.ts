import type { HealthState, MonitorCoverage, MonitorMatrixBucket, MonitorProbeObservation } from '@/api/channelMonitorV2'
import { healthModeState, type HealthDisplayMode } from './monitorFormat'

export interface GroupHistorySlot {
  start: number
  end: number
  bucket?: MonitorMatrixBucket
  probe?: MonitorProbeObservation
  carried?: boolean
}

export const GROUP_CARD_HISTORY_SLOTS = 12

export function groupHistoryState(slot: GroupHistorySlot, mode: HealthDisplayMode): HealthState {
  if (slot.probe) {
    if (slot.probe.status === 'operational') return 'healthy'
    if (slot.probe.status === 'degraded') return 'warning'
    return 'critical'
  }
  return slot.bucket ? healthModeState(slot.bucket.health, mode) : 'unknown'
}

/** Show the latest observations, not elapsed wall-clock intervals. Idle time
 * never pushes a result out of the strip. Repeated padding is explicitly marked
 * as carried state rather than presented as extra measurements. */
export function groupCardHistory(
  buckets: MonitorMatrixBucket[], coverage: MonitorCoverage, _mode: HealthDisplayMode,
  fallback?: MonitorMatrixBucket, probes: MonitorProbeObservation[] = [],
): GroupHistorySlot[] {
  const nativeMs = Math.max(1, coverage.bucket_seconds || 60) * 1000
  const unique = new Map<string, GroupHistorySlot>()
  for (const bucket of [...(fallback ? [fallback] : []), ...buckets]) {
    const start = Date.parse(bucket.bucket_start)
    if (Number.isFinite(start)) unique.set(`request:${start}`, { start, end: start + nativeMs, bucket })
  }
  for (const probe of probes) {
    const start = Date.parse(probe.checked_at)
    if (Number.isFinite(start)) unique.set(`probe:${start}`, { start, end: start, probe })
  }
  const observations = [...unique.values()].sort((a, b) => a.start - b.start).slice(-GROUP_CARD_HISTORY_SLOTS)
  if (!observations.length) {
    return Array.from({ length: GROUP_CARD_HISTORY_SLOTS }, () => ({ start: 0, end: 0 }))
  }
  const first = observations[0]!
  return [
    ...Array.from({ length: GROUP_CARD_HISTORY_SLOTS - observations.length }, () => ({ ...first, carried: true })),
    ...observations,
  ]
}
