import type { MonitorCoverage, MonitorMatrixBucket } from '@/api/channelMonitorV2'
import { healthModeScore, healthModeState, type HealthDisplayMode } from './monitorFormat'

export interface GroupHistorySlot {
  start: number
  end: number
  bucket?: MonitorMatrixBucket
}

export const GROUP_CARD_HISTORY_SLOTS = 7

/** Compact presentation only: always render seven display cells. Each cell keeps the
 * worst observed native bucket in its interval; when an interval has no fresh
 * request, carry the previous observed state forward so an idle group does not
 * visually collapse or turn into a sequence of meaningless empty gaps. Never
 * average percentile values. */
export function groupCardHistory(buckets: MonitorMatrixBucket[], coverage: MonitorCoverage, mode: HealthDisplayMode, fallback?: MonitorMatrixBucket): GroupHistorySlot[] {
  const from = Date.parse(coverage.requested_start)
  const to = Date.parse(coverage.requested_end || coverage.data_through)
  if (!Number.isFinite(from) || !Number.isFinite(to) || to <= from) return []

  // Keep the seven display intervals stable regardless of the native sampling
  // cadence (5m/1h/12h/1d). Integer boundaries avoid cumulative rounding drift.
  const slots: GroupHistorySlot[] = Array.from({ length: GROUP_CARD_HISTORY_SLOTS }, (_, index) => ({
    start: from + Math.floor((to - from) * index / GROUP_CARD_HISTORY_SLOTS),
    end: from + Math.floor((to - from) * (index + 1) / GROUP_CARD_HISTORY_SLOTS),
  }))
  const severity = { unknown: -1, healthy: 0, warning: 1, critical: 2 }
  const sorted = buckets
    .map(bucket => ({ bucket, time: Date.parse(bucket.bucket_start) }))
    .filter(item => Number.isFinite(item.time) && item.time >= from && item.time < to)
    .sort((a, b) => a.time - b.time)
  let cursor = 0
  let carried: MonitorMatrixBucket | undefined = fallback
  for (const slot of slots) {
    let selected: MonitorMatrixBucket | undefined
    while (cursor < sorted.length && sorted[cursor]!.time < slot.end) {
      const candidate = sorted[cursor]!.bucket
      if (sorted[cursor]!.time >= slot.start) {
        const score = healthModeScore(candidate.health, mode)
        const currentScore = selected ? healthModeScore(selected.health, mode) : null
        const rank = severity[healthModeState(candidate.health, mode)]
        const currentRank = selected ? severity[healthModeState(selected.health, mode)] : -1
        if (!selected || rank > currentRank || (rank === currentRank && score != null && (currentScore == null || score < currentScore))) selected = candidate
      }
      cursor += 1
    }
    const display = selected || carried
    if (display) {
      slot.bucket = display
      carried = display
    }
  }
  return slots
}
