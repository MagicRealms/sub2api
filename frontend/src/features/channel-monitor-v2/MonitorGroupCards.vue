<template>
  <div class="monitor-group-cards" data-testid="monitor-group-cards">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
      <p>{{ t('channelMonitorV2.cards.description') }}</p>
      <span class="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">
        <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" />{{ t('channelMonitorV2.cards.passive') }}
      </span>
    </div>
    <div v-if="entries.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article v-for="{ card, slots } in entries" :key="card.group_id" class="monitor-group-card min-w-0 rounded-3xl border border-gray-200/80 bg-white p-4 transition-shadow hover:shadow-md dark:border-dark-600/70 dark:bg-dark-800 sm:p-5" :data-group-id="card.group_id">
        <header class="flex items-start gap-3">
          <span class="group-icon flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ring-1 ring-inset ring-black/5 dark:ring-white/10" :class="platformBadgeLightClass(card.platform)" :title="providerName(card.platform)">
            <PlatformIcon :platform="card.platform as GroupPlatform" size="lg" class="!h-6 !w-6" />
          </span>
          <div class="min-w-0 flex-1">
            <h2 class="truncate text-sm font-bold text-gray-900 dark:text-gray-100" :title="card.group_name">{{ card.group_name || `#${card.group_id}` }}</h2>
            <span class="mt-1 inline-block text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ providerName(card.platform) }}</span>
            <span v-if="card.health.sample_limited" class="ml-1.5 text-[10px] text-gray-400" :title="t('channelMonitorV2.cards.limitedHint')">{{ t('channelMonitorV2.cards.limited') }}</span>
          </div>
          <span class="inline-flex shrink-0 items-center gap-1.5 text-[11px] font-medium text-gray-600 dark:text-gray-300">
            <i class="status-dot" :class="cellClass(card.health, card.has_traffic)" />{{ statusLabel(card) }}
          </span>
        </header>
        <div class="mt-3 flex min-w-0 items-center gap-1.5 rounded-lg bg-gray-50 px-2.5 py-1.5 dark:bg-dark-900/50" :title="t('channelMonitorV2.cards.selectionHint')">
          <Icon name="cube" size="sm" class="shrink-0 text-gray-400" />
          <span class="min-w-0 truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="modelLabel(card)">{{ modelLabel(card) }}</span>
          <span v-if="card.has_cache_samples" class="ml-auto shrink-0 text-[10px] text-primary-600 dark:text-primary-300">{{ t('channelMonitorV2.cards.bestCache') }}</span>
        </div>
        <div v-if="card.probe" class="mt-2 flex items-center justify-between gap-2 rounded-lg border border-primary-100 bg-primary-50/70 px-2.5 py-1.5 text-[11px] dark:border-primary-900/40 dark:bg-primary-900/20">
          <span class="truncate text-primary-700 dark:text-primary-200" :title="card.probe.message || undefined">{{ t('channelMonitorV2.cards.hourlyProbe', { model: card.probe.model || '—' }) }}</span>
          <span class="shrink-0 font-semibold" :class="probeClass(card.probe.status)">{{ probeLabel(card.probe.status) }}<template v-if="card.probe.latency_ms != null"> · {{ card.probe.latency_ms }}ms</template></span>
        </div>
        <dl class="mt-4 grid grid-cols-3 gap-2">
          <div class="rounded-2xl border border-gray-100 bg-gray-50/60 px-2.5 py-3 dark:border-dark-700 dark:bg-dark-900/30">
            <dt class="min-h-7 text-[10px] leading-[14px] text-gray-500 dark:text-gray-400">{{ t('channelMonitorV2.metrics.successRate') }}</dt>
            <dd class="card-metric-value mt-1.5 font-bold tabular-nums text-gray-900 dark:text-gray-100">{{ card.has_traffic ? formatMonitorPercent(1 - card.metrics.error_rate) : '—' }}</dd>
          </div>
          <div class="rounded-2xl border border-gray-100 bg-gray-50/60 px-2.5 py-3 dark:border-dark-700 dark:bg-dark-900/30" :title="formatLatencyPrivacy(card.metrics.ttft.p50_ms, card.metrics.ttft.p90_ms, card.metrics.ttft.avg_ms, card.metrics.ttft.p95_ms)">
            <dt class="min-h-7 text-[10px] leading-[14px] text-gray-500 dark:text-gray-400">{{ t('channelMonitorV2.metrics.ttftP50') }}</dt>
            <dd class="card-metric-value mt-1.5 font-bold tabular-nums text-gray-900 dark:text-gray-100">{{ card.has_traffic ? formatMonitorMs(card.metrics.ttft.p50_ms) : '—' }}</dd>
          </div>
          <div class="rounded-2xl border border-gray-100 bg-gray-50/60 px-2.5 py-3 dark:border-dark-700 dark:bg-dark-900/30">
            <dt class="min-h-7 text-[10px] leading-[14px] text-gray-500 dark:text-gray-400">{{ t('channelMonitorV2.metrics.cacheRate') }}</dt>
            <dd class="card-metric-value mt-1.5 font-bold tabular-nums text-gray-900 dark:text-gray-100">{{ card.has_cache_samples ? formatMonitorPercent(card.metrics.cache_rate) : '—' }}</dd>
          </div>
        </dl>
        <div class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700">
          <div class="mb-2 flex items-center justify-between gap-2 text-[10px] text-gray-500 dark:text-gray-400">
            <span>{{ t('channelMonitorV2.cards.window', { range: rangeLabel }) }}</span>
            <span>{{ t('channelMonitorV2.cards.historyLabel') }}</span>
          </div>
          <div class="mb-2 flex items-center gap-3 text-[10px] text-gray-500 dark:text-gray-400" aria-hidden="true">
            <span v-for="state in ['healthy', 'warning', 'critical', 'unknown']" :key="state" class="inline-flex items-center gap-1"><i class="status-dot" :class="`health-${state}`" />{{ t(`channelMonitorV2.cards.${state}`) }}</span>
          </div>
          <div class="group-history flex h-5 items-stretch gap-[3px]" :aria-label="t('channelMonitorV2.cards.historyHint')">
            <button v-for="slot in slots" :key="slot.start" type="button" class="min-w-0 flex-1 rounded-full transition-opacity hover:opacity-70 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500" :class="slot.bucket ? cellClass(slot.bucket.health, true) : 'health-unknown'" :aria-label="slotLabel(slot)" @mouseenter="showTooltip($event, slot)" @mouseleave="closeTooltip" @focus="showTooltip($event, slot)" @blur="closeTooltip" @click="showTooltip($event, slot)" />
          </div>
          <div class="mt-1.5 flex justify-between text-[10px] text-gray-400 dark:text-gray-500">
            <span>{{ time(coverage.requested_start) }}</span><span>{{ time(coverage.requested_end || coverage.data_through) }}</span>
          </div>
        </div>
      </article>
    </div>
    <p v-else class="py-12 text-center text-sm text-gray-500">{{ t('channelMonitorV2.empty.title') }}</p>
    <p class="mt-4 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">{{ t('channelMonitorV2.cards.historyHint') }}</p>
    <FloatingPanel :open="!!tooltipSlot" :anchor="tooltipAnchor" :min-width="220" :max-height="300" role="tooltip" class="group-history-tooltip rounded-2xl border border-gray-200 bg-white p-3 text-xs leading-relaxed text-gray-700 shadow-xl dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200" @close="closeTooltip">
      <template v-if="tooltipSlot">
        <p class="font-semibold">{{ time(tooltipSlot.start) }} — {{ time(tooltipSlot.end) }}</p>
        <template v-if="tooltipSlot.bucket">
          <p class="font-medium" :class="tooltipSlot.bucket.health.sample_limited ? 'text-amber-700 dark:text-amber-300' : 'text-gray-500'">{{ t(`channelMonitorV2.cards.${healthModeState(tooltipSlot.bucket.health, healthMode)}`) }}</p>
          <p v-if="tooltipSlot.bucket.health.sample_limited" class="my-1 text-gray-500 dark:text-gray-400">{{ t('channelMonitorV2.cards.limitedHint') }}</p>
          <p class="mb-2 text-gray-400">{{ t('channelMonitorV2.cards.sampleAt', { time: time(tooltipSlot.bucket.bucket_start) }) }}</p>
          <p>{{ t('channelMonitorV2.metrics.successRateValue', { value: formatMonitorPercent(1 - tooltipSlot.bucket.metrics.error_rate) }) }}</p>
          <p>{{ t('channelMonitorV2.metrics.ttftValue', { value: formatMonitorMs(tooltipSlot.bucket.metrics.ttft.p50_ms) }) }}</p>
          <p>{{ t('channelMonitorV2.metrics.cacheRateValue', { value: formatMonitorPercent(tooltipSlot.bucket.metrics.cache_rate) }) }}</p>
        </template>
        <p v-else>{{ t('channelMonitorV2.matrix.noTraffic') }}</p>
      </template>
    </FloatingPanel>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorCoverage, MonitorGroupCard, MonitorHealth } from '@/api/channelMonitorV2'
import type { GroupPlatform } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import FloatingPanel from '@/components/common/FloatingPanel.vue'
import { platformBadgeLightClass } from '@/utils/platformColors'
import { formatLatencyPrivacy, formatMonitorMs, formatMonitorPercent, healthModeState, type HealthDisplayMode } from './monitorFormat'
import { groupCardHistory, type GroupHistorySlot } from './groupCardHistory'

const props = defineProps<{ cards: MonitorGroupCard[]; coverage: MonitorCoverage; healthMode: HealthDisplayMode; rangeLabel: string }>()
const { t, locale } = useI18n()
const entries = computed(() => props.cards.map(card => ({ card, slots: groupCardHistory(card.buckets, props.coverage, props.healthMode) })))
const tooltipSlot = ref<GroupHistorySlot | null>(null)
const tooltipAnchor = ref<HTMLElement | null>(null)
function closeTooltip() { tooltipSlot.value = null; tooltipAnchor.value = null }
function showTooltip(event: MouseEvent | FocusEvent, slot: GroupHistorySlot) {
  tooltipAnchor.value = event.currentTarget as HTMLElement
  tooltipSlot.value = slot
}
watch(() => [props.cards, props.healthMode, props.coverage], closeTooltip)
function cellClass(health: MonitorHealth, sampled: boolean) { return `health-${sampled ? healthModeState(health, props.healthMode) : 'unknown'}` }
function modelLabel(card: MonitorGroupCard) {
  if (!card.has_traffic || !card.model) return t('channelMonitorV2.cards.noSamples')
  return card.model === '__other__' ? t('channelMonitorV2.otherModels') : card.model
}
function providerName(platform: string) {
  const names: Record<string,string> = { openai: 'OpenAI', anthropic: 'Anthropic', gemini: 'Gemini', grok: 'Grok', kimi: 'Kimi', deepseek: 'DeepSeek', zhipu: 'GLM', kiro: 'Kiro', antigravity: 'Antigravity' }
  return names[platform] || platform || '—'
}
function statusLabel(card: MonitorGroupCard) {
  if (!card.has_traffic) return t('channelMonitorV2.cards.noTraffic')
  return t(`channelMonitorV2.cards.${healthModeState(card.health, props.healthMode)}`)
}
function probeClass(status?: string) {
  if (status === 'operational') return 'text-emerald-600 dark:text-emerald-300'
  if (status === 'degraded') return 'text-amber-600 dark:text-amber-300'
  if (status) return 'text-rose-600 dark:text-rose-300'
  return 'text-gray-500 dark:text-gray-400'
}
function probeLabel(status?: string) {
  if (status === 'operational') return t('channelMonitorV2.cards.probeOperational')
  if (status === 'degraded') return t('channelMonitorV2.cards.probeDegraded')
  if (status) return t('channelMonitorV2.cards.probeFailed')
  return t('channelMonitorV2.cards.probePending')
}
function time(value: string | number) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value || 'zh-CN', { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
}
function slotLabel(slot: GroupHistorySlot) {
  return `${time(slot.start)} — ${time(slot.end)} · ${slot.bucket ? t('channelMonitorV2.cards.sampleAt', { time: time(slot.bucket.bucket_start) }) : t('channelMonitorV2.matrix.noTraffic')}`
}
</script>

<style scoped>
.monitor-group-card { container-type: inline-size; }
.card-metric-value { font-size: 14px; font-size: clamp(12px, 5cqi, 18px); }
.status-dot { display: inline-block; width: 6px; height: 6px; border-radius: 999px; }
.health-healthy { background: #10b981; }
.health-warning { background: #f59e0b; }
.health-critical { background: #ef4444; }
.health-unknown { background: #d1d5db; }
:global(.dark .monitor-group-cards .health-unknown) { background: #49596d; }
@media (prefers-reduced-motion: reduce) { .monitor-group-card { transition: none; } }
</style>
