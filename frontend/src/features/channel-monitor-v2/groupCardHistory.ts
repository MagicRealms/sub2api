import type { MonitorCoverage, MonitorMatrixBucket } from '@/api/channelMonitorV2'
import { healthModeScore, healthModeState, type HealthDisplayMode } from './monitorFormat'

export interface GroupHistorySlot {
  start: number
  end: number
  bucket?: MonitorMatrixBucket
}

/** Compact presentation only: retain V2 sampling and show the worst observed state / scored
 * native bucket in each display interval. Never average percentile values. */
export function groupCardHistory(buckets: MonitorMatrixBucket[], coverage: MonitorCoverage, mode: HealthDisplayMode): GroupHistorySlot[] {
  const from = Date.parse(coverage.requested_start)
  const to = Date.parse(coverage.requested_end || coverage.data_through)
  const nativeStep = Math.max(60, coverage.bucket_seconds || 60) * 1000
  if (!Number.isFinite(from) || !Number.isFinite(to) || to <= from) return []
  const aligned = Math.floor(from / nativeStep) * nativeStep
  const nativeCount = Math.ceil((to - aligned) / nativeStep)
  const step = nativeStep * Math.max(1, Math.ceil(nativeCount / 36))
  const slots: GroupHistorySlot[] = []
  for (let start = aligned; start < to; start += step) slots.push({ start, end: Math.min(to, start + step) })
  for (const bucket of buckets) {
    const time = Date.parse(bucket.bucket_start)
    if (!Number.isFinite(time) || time < aligned || time >= to) continue
    const slot = slots[Math.floor((time - aligned) / step)]!
    const score = healthModeScore(bucket.health, mode)
    const currentScore = slot.bucket ? healthModeScore(slot.bucket.health, mode) : null
    const severity = { unknown: -1, healthy: 0, warning: 1, critical: 2 }
    const rank = severity[healthModeState(bucket.health, mode)]
    const currentRank = slot.bucket ? severity[healthModeState(slot.bucket.health, mode)] : -1
    if (!slot.bucket || rank > currentRank || (rank === currentRank && score != null && (currentScore == null || score < currentScore))) slot.bucket = bucket
  }
  // An idle interval is not a health state. Only display intervals with an
  // observed bucket so no-traffic periods do not appear as gray samples.
  return slots.filter((slot) => slot.bucket != null)
}
