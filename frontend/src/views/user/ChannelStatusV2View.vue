<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <!-- Ops-style elevated shell: title toolbar + filters (mirrors OpsDashboardHeader) -->
      <section
        class="card !rounded-3xl !border-0 p-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
      >
        <header class="page-header monitor-overview-header mb-0 flex flex-col items-start gap-5 xl:flex-row xl:items-center border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <div class="min-w-0 shrink-0 xl:w-56">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
                <Icon name="chart" size="sm" />
              </span>
              {{ t('channelMonitorV2.title') }}
            </h1>
            <div class="page-description mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="relative flex h-2 w-2 shrink-0">
                <span
                  class="relative inline-flex h-2 w-2 rounded-full"
                  :class="loading || refreshing ? 'bg-gray-400' : 'bg-green-500'"
                ></span>
              </span>
              <span v-if="refreshing" class="inline-flex items-center gap-1 text-primary-600 dark:text-primary-300">
                <LoadingSpinner size="sm" />
                {{ t('channelMonitorV2.updating') }}
              </span>
              <span v-else-if="snapshot?.coverage.data_through">
                {{ t('channelMonitorV2.updatedTo', { time: formatTime(snapshot.coverage.data_through) }) }}
              </span>
              <span v-else class="text-gray-400">{{ t('common.loading') }}</span>
              <span
                v-if="snapshot && !snapshot.coverage.coverage_complete && !bootstrapActive"
                class="badge badge-warning"
              >
                {{ t('channelMonitorV2.partialCoverage') }}
              </span>
              <span
                v-if="bootstrapActive"
                class="badge badge-primary inline-flex items-center gap-1"
              >
                <LoadingSpinner size="sm" />
                {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
              </span>
            </div>
          </div>

          <section
            v-if="snapshot"
            class="monitor-summary grid w-full min-w-0 grid-cols-2 gap-2 sm:grid-cols-3 xl:w-auto xl:flex-1"
            :class="showThroughput ? 'lg:grid-cols-5' : 'sm:grid-cols-3'"
            :aria-label="t('channelMonitorV2.summaryAria')"
          >
            <MetricCell
              compact
              :label="t('channelMonitorV2.metrics.successRate')"
              :value="formatPercent(1 - snapshot.metrics.error_rate)"
              :detail="t('channelMonitorV2.metrics.errorRateValue', { value: formatPercent(snapshot.metrics.error_rate) })"
              :state="snapshot.health.error_rate"
            />
            <MetricCell
              compact
              :label="t('channelMonitorV2.metrics.ttftP50')"
              :value="formatMs(snapshot.metrics.ttft.p50_ms)"
              :detail="latencyKpiSecondary(snapshot.metrics.ttft)"
              :title="latencyDetail(snapshot.metrics.ttft)"
              :state="ttftCellState(snapshot.health.ttft, snapshot.metrics.ttft)"
            />
            <MetricCell
              compact
              v-if="showThroughput"
              :label="t('channelMonitorV2.metrics.tps')"
              :value="formatTps(snapshot.metrics.tpm)"
              :detail="t('channelMonitorV2.metrics.tpsDetail')"
              :title="exactTps(snapshot.metrics.tpm)"
            />
            <MetricCell
              compact
              :label="t('channelMonitorV2.metrics.cacheRate')"
              :value="formatPercent(snapshot.metrics.cache_rate)"
              :detail="t('channelMonitorV2.metrics.cacheDetail')"
              :state="snapshot.health.cache || snapshot.health.overall"
            />
            <MetricCell
              compact
              v-if="showThroughput"
              :label="t('channelMonitorV2.metrics.rpm')"
              :value="formatRate(snapshot.metrics.rpm)"
              :detail="t('channelMonitorV2.metrics.rpmDetail')"
              :title="exactRate(snapshot.metrics.rpm)"
            />
          </section>
          <section
            v-else-if="loading"
            class="monitor-summary grid w-full min-w-0 grid-cols-2 gap-2 sm:grid-cols-3 xl:w-auto xl:flex-1"
            :class="showThroughput ? 'lg:grid-cols-5' : 'sm:grid-cols-3'"
            aria-hidden="true"
          >
            <div
              v-for="i in (showThroughput ? 5 : 3)"
              :key="i"
              class="h-24 animate-pulse rounded-2xl bg-gray-50 dark:bg-dark-900/30"
            />
          </section>
        </header>

        <!-- First-upgrade silent backfill: show until 30d product window is covered -->
        <div
          v-if="bootstrapActive"
          class="border-b border-blue-100 bg-blue-50/90 px-5 py-3 dark:border-blue-900/40 dark:bg-blue-950/40 sm:px-6"
          role="status"
          aria-live="polite"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <p class="text-sm font-semibold text-blue-900 dark:text-blue-100">
                {{ t('channelMonitorV2.bootstrap.title') }}
              </p>
              <p class="mt-0.5 text-xs text-blue-800/80 dark:text-blue-200/80">
                {{ t('channelMonitorV2.bootstrap.description') }}
              </p>
            </div>
            <span class="shrink-0 text-xs font-medium tabular-nums text-blue-700 dark:text-blue-300">
              {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
            </span>
          </div>
          <div
            class="mt-2.5 h-1.5 overflow-hidden rounded-full bg-blue-200/80 dark:bg-blue-900/60"
            role="progressbar"
            :aria-valuenow="bootstrapPercent"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-label="t('channelMonitorV2.bootstrap.working')"
          >
            <div
              class="h-full rounded-full bg-blue-500 transition-[width] duration-500 ease-out dark:bg-blue-400"
              :style="{ width: `${bootstrapPercent}%` }"
            />
          </div>
        </div>

        <!-- Compact controls wrap on narrow screens so health modes stay together. -->
        <div class="monitor-toolbar flex flex-wrap items-center gap-1.5 px-4 py-3 sm:gap-2 sm:px-5">
          <div
            class="tabs inline-flex shrink-0"
            role="group"
            :aria-label="t('channelMonitorV2.timeRange')"
          >
            <button
              v-for="option in ranges"
              :key="option.value"
              type="button"
              class="tab !px-2 !py-1 text-xs sm:!px-2.5"
              :class="filter.range === option.value ? 'tab-active' : ''"
              @click="setRange(option.value)"
            >
              {{ option.label }}
            </button>
          </div>

          <span class="mx-0.5 hidden h-5 w-px shrink-0 bg-gray-200 dark:bg-dark-700 sm:block" aria-hidden="true"></span>

          <FilterMultiSelect
            v-model="filter.platforms"
            compact
            :label="t('channelMonitorV2.filters.platform')"
            :all-label="t('channelMonitorV2.filters.allPlatforms')"
            :options="platformOptions"
          />
          <FilterMultiSelect
            v-model="selectedGroupIds"
            compact
            :label="t('channelMonitorV2.filters.group')"
            :all-label="t('channelMonitorV2.filters.allGroups')"
            :options="groupOptions"
          />
          <FilterMultiSelect
            v-model="filter.models"
            compact
            :label="t('channelMonitorV2.filters.model')"
            :all-label="t('channelMonitorV2.filters.allModels')"
            :options="modelOptions"
          />
          <button
            type="button"
            class="btn btn-ghost btn-sm shrink-0 !px-2 !py-1 text-xs"
            :disabled="!hasDimensionFilter"
            :class="!hasDimensionFilter ? 'opacity-40' : ''"
            @click="clearDimensions"
          >
            {{ t('channelMonitorV2.clearFilters') }}
          </button>

          <span class="mx-0.5 hidden h-5 w-px shrink-0 bg-gray-200 dark:bg-dark-700 md:block" aria-hidden="true"></span>

          <div
            class="monitor-health-controls inline-flex shrink-0 items-center gap-2"
          >
            <div
              class="tabs inline-flex"
              role="group"
              :aria-label="t('channelMonitorV2.healthMode.label')"
            >
              <button
                v-for="option in healthModeOptions"
                :key="option.value"
                type="button"
                class="tab !px-2 !py-1 text-xs"
                :class="healthMode === option.value ? 'tab-active' : ''"
                @click="healthMode = option.value"
              >
                {{ option.label }}
              </button>
            </div>
            <button
              class="btn btn-secondary btn-icon shrink-0 flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
              type="button"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              :disabled="loading || refreshing"
              @click="reload(false)"
            >
              <Icon name="refresh" size="sm" :class="loading || refreshing ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
        <div class="border-t border-gray-100 p-4 dark:border-dark-700 sm:p-5" :aria-busy="loading || refreshing">
          <MonitorGroupCards
            v-if="cards"
            :cards="groupCards"
            :coverage="cards.coverage"
            :health-mode="healthMode"
            :range-label="rangeLabel"
            :scope-key="JSON.stringify([filter.platforms, filter.groupIds, filter.models])"
          />
          <div v-else-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-hidden="true">
            <div v-for="index in 6" :key="index" class="h-64 animate-pulse rounded-3xl bg-gray-100 dark:bg-dark-700" />
          </div>
          <p v-else class="py-12 text-center text-sm text-gray-500">{{ t('channelMonitorV2.empty.title') }}</p>
        </div>
      </section>


    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import FilterMultiSelect from '@/features/channel-monitor-v2/FilterMultiSelect.vue'
import MetricCell from '@/features/channel-monitor-v2/MetricCell.vue'
import MonitorGroupCards from '@/features/channel-monitor-v2/MonitorGroupCards.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isChannelMonitorThroughputHidden } from '@/utils/featureFlags'
import * as api from '@/api/channelMonitorV2'
import type {
  HealthState,
  MonitorDimensions,
  MonitorFilter,
  MonitorGroupCard,
  MonitorGroupCardsResponse,
  MonitorRange,
  MonitorSnapshot,
} from '@/api/channelMonitorV2'
import {
  formatLatencyKpiSecondary,
  formatLatencyPrivacy,
  formatMonitorMs,
  formatMonitorPercent,
  formatMonitorThroughput,
  formatMonitorTokensPerSecond,
  tokensPerSecondFromTpm,
  ttftDisplayState,
} from '@/features/channel-monitor-v2/monitorFormat'

type HealthMode = 'overall' | 'success' | 'ttft' | 'cache'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const { t, locale } = useI18n()
const isAdmin = computed(() => authStore.isAdmin)
/** Admins always see RPM/TPM; users honor the hide-throughput system setting. */
const showThroughput = computed(() => isAdmin.value || !isChannelMonitorThroughputHidden())
const ranges = computed(() => [
  { value: '90m' as MonitorRange, label: t('channelMonitorV2.ranges.90m') },
  { value: '24h' as MonitorRange, label: t('channelMonitorV2.ranges.24h') },
  { value: '7d' as MonitorRange, label: t('channelMonitorV2.ranges.7d') },
  { value: '30d' as MonitorRange, label: t('channelMonitorV2.ranges.30d') },
])
const healthModeOptions = computed(() => [
  { value: 'overall' as HealthMode, label: t('channelMonitorV2.healthMode.overall') },
  { value: 'success' as HealthMode, label: t('channelMonitorV2.healthMode.success') },
  { value: 'ttft' as HealthMode, label: t('channelMonitorV2.healthMode.ttft') },
  { value: 'cache' as HealthMode, label: t('channelMonitorV2.healthMode.cache') },
])

const filter = ref<MonitorFilter>({
  range: parseRange(route.query.range),
  platforms: csv(route.query.platform),
  groupIds: csv(route.query.group).map(Number).filter(Boolean),
  models: csv(route.query.model),
})
const healthMode = ref<HealthMode>(parseHealthMode(route.query.health_mode))
const dimensions = ref<MonitorDimensions>({ platforms: [], groups: [], models: [] })
const snapshot = ref<MonitorSnapshot | null>(null)
const cards = ref<MonitorGroupCardsResponse | null>(null)
const loading = ref(false)
const refreshing = ref(false)
let controller: AbortController | null = null
let sequence = 0
let autoRefreshTimer: number | null = null

const hasDimensionFilter = computed(
  () => filter.value.platforms.length + filter.value.groupIds.length + filter.value.models.length > 0
)
// Full platform catalog (never pruned). Groups/models cascade by selected platforms
// so choosing a platform narrows the other pickers without collapsing platforms.
const platformOptions = computed(() =>
  (dimensions.value.platforms || []).map((item) => ({
    value: item.value,
    label: item.label,
  }))
)
const selectedPlatforms = computed(() => new Set(filter.value.platforms))
const groupOptions = computed(() =>
  (dimensions.value.groups || [])
    .filter(
      (item) =>
        selectedPlatforms.value.size === 0 ||
        !item.platform ||
        selectedPlatforms.value.has(item.platform),
    )
    .map((item) => ({
      value: String(item.id),
      label: item.platform ? `${item.platform} / ${item.name || `#${item.id}`}` : item.name || `#${item.id}`,
    }))
)
const modelOptions = computed(() =>
  (dimensions.value.models || [])
    .filter(
      (item) =>
        selectedPlatforms.value.size === 0 ||
        !item.platform ||
        selectedPlatforms.value.has(item.platform),
    )
    .map((item) => ({
      value: item.value,
      label:
        item.platform && !item.label.includes(item.platform)
          ? `${item.platform} / ${item.label}`
          : item.label,
    }))
)
const selectedGroupIds = computed({
  get: () => filter.value.groupIds.map(String),
  set: (value: string[]) => {
    filter.value.groupIds = value.map(Number).filter((id) => Number.isInteger(id) && id > 0)
  },
})
// Soft-prune group/model selections that fall outside the platform cascade.
// Do NOT wipe when options are temporarily empty (loading); only drop invalid ids.
watch(
  [groupOptions, modelOptions],
  () => {
    if (groupOptions.value.length > 0) {
      const allowed = new Set(groupOptions.value.map((item) => item.value))
      const next = filter.value.groupIds.filter((id) => allowed.has(String(id)))
      if (next.length !== filter.value.groupIds.length) {
        filter.value.groupIds = next
      }
    }
    if (modelOptions.value.length > 0) {
      const allowed = new Set(modelOptions.value.map((item) => item.value))
      const next = filter.value.models.filter((model) => allowed.has(model))
      if (next.length !== filter.value.models.length) {
        filter.value.models = next
      }
    }
  },
  { flush: 'post' },
)
/** First-upgrade backfill toward 90m/24h/7d/30d; banner hides when backend omits bootstrap. */
const bootstrapActive = computed(() => Boolean(snapshot.value?.coverage?.bootstrap?.active))
const bootstrapPercent = computed(() => {
  const raw = snapshot.value?.coverage?.bootstrap?.progress_percent
  if (typeof raw !== 'number' || Number.isNaN(raw)) return 0
  return Math.min(100, Math.max(0, Math.round(raw)))
})
const rangeLabel = computed(() => t(`channelMonitorV2.ranges.${filter.value.range}`))
const groupCards = computed<MonitorGroupCard[]>(() => {
  const history = cards.value?.probe_history_by_group || {}
  const rows = (cards.value?.items || []).map(row => ({
    ...row,
    probe_history: row.group_id != null ? history[String(row.group_id)] || row.probe_history : row.probe_history,
  }))
  const present = new Set(rows.map(row => row.group_id))
  // The scoped catalog also includes configured groups with no sampled platform.
  // Show them explicitly as empty instead of silently omitting the group.
  for (const group of dimensions.value.groups) {
    if (present.has(group.id) ||
        (filter.value.groupIds.length && !filter.value.groupIds.includes(group.id)) ||
        (filter.value.platforms.length && !filter.value.platforms.includes(group.platform || ''))) continue
    rows.push({
      platform: group.platform || '', group_id: group.id, group_name: group.name,
      has_traffic: false, has_cache_samples: false, buckets: [],
      probe_history: history[String(group.id)],
      metrics: { success_requests: 0, error_requests: 0, request_count: 0, token_count: 0,
        rpm: 0, tpm: 0, error_rate: 0, cache_rate: 0, cache_rate_numerator: 0, cache_rate_denominator: 0,
        ttft: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
        duration: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null } },
      health: { overall: 'unknown', error_rate: 'unknown', ttft: 'unknown', cache: 'unknown', minimum_sample: 0 },
    })
  }
  return rows.sort((a,b) => Number(b.has_traffic) - Number(a.has_traffic) || (a.group_id || 0) - (b.group_id || 0))
})

function csv(value: unknown) {
  return typeof value === 'string' ? value.split(',').filter(Boolean) : []
}
function parseRange(value: unknown): MonitorRange {
  return ['90m', '24h', '7d', '30d'].includes(String(value)) ? (value as MonitorRange) : '90m'
}
function parseHealthMode(value: unknown): HealthMode {
  const allowed: HealthMode[] = ['overall', 'success', 'ttft', 'cache']
  return allowed.includes(value as HealthMode) ? (value as HealthMode) : 'overall'
}
function syncQuery() {
  void router.replace({
    query: {
      range: filter.value.range,
      platform: filter.value.platforms.join(',') || undefined,
      group: filter.value.groupIds.join(',') || undefined,
      model: filter.value.models.join(',') || undefined,
      health_mode: healthMode.value,
    },
  })
}
/** Dimensions catalog: range only — never re-filtered by platform/group/model selection. */
async function loadDimensions(signal?: AbortSignal, id = sequence) {
  const rangeOnly: MonitorFilter = {
    range: filter.value.range,
    platforms: [],
    groupIds: [],
    models: [],
  }
  const next = await api.getDimensions(rangeOnly, isAdmin.value, signal)
  if (id !== sequence) return
  dimensions.value = next
}

async function loadMetrics(signal?: AbortSignal, id = sequence) {
  const [nextSnapshot, nextCards] = await Promise.all([
    api.getSnapshot(filter.value, isAdmin.value, signal),
    api.getGroupCards(filter.value, isAdmin.value, signal),
  ])
  if (id !== sequence) return
  snapshot.value = nextSnapshot
  cards.value = nextCards
  scheduleAutoRefresh()
}

async function reload(silent = true) {
  controller?.abort()
  const request = new AbortController()
  controller = request
  const id = ++sequence
  refreshing.value = true
  if (!silent) loading.value = true
  try {
    // Catalog + metrics in parallel; catalog ignores dimension filters so options never shrink.
    await Promise.all([
      loadDimensions(request.signal, id),
      loadMetrics(request.signal, id),
    ])
  } catch (error) {
    if ((error as { name?: string }).name !== 'CanceledError') {
      appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
    }
  } finally {
    if (id === sequence) {
      loading.value = false
      refreshing.value = false
    }
  }
}

/** When only range changes, still refresh dimensions; dimension filters only re-load metrics. */
async function reloadMetricsOnly(silent = true) {
  controller?.abort()
  const request = new AbortController()
  controller = request
  const id = ++sequence
  refreshing.value = true
  if (!silent) loading.value = true
  try {
    await loadMetrics(request.signal, id)
  } catch (error) {
    if ((error as { name?: string }).name !== 'CanceledError') {
      appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
    }
  } finally {
    if (id === sequence) {
      loading.value = false
      refreshing.value = false
    }
  }
}
function setRange(value: MonitorRange) {
  filter.value.range = value
}
function clearDimensions() {
  // Replace arrays so deep watch always fires and metrics reload full window.
  filter.value = {
    ...filter.value,
    platforms: [],
    groupIds: [],
    models: [],
  }
}
function scheduleAutoRefresh() {
  if (autoRefreshTimer) {
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
  // Poll faster while first-upgrade bootstrap is filling 90m→30d so the progress bar moves.
  const seconds = bootstrapActive.value
    ? 10
    : snapshot.value?.config?.refresh_interval_seconds || 300
  autoRefreshTimer = window.setInterval(() => {
    if (!loading.value && !refreshing.value) {
      void reload(true)
    }
  }, Math.max(bootstrapActive.value ? 10 : 60, seconds) * 1000)
}
function formatRate(value: number) {
  return formatMonitorThroughput(value)
}
function exactRate(value: number) {
  return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 2 }).format(value || 0)
}
function formatTps(tpm: number | null | undefined) {
  return formatMonitorTokensPerSecond(tpm)
}
function exactTps(tpm: number | null | undefined) {
  return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 3 }).format(
    tokensPerSecondFromTpm(tpm),
  )
}
function formatPercent(value: number) {
  return formatMonitorPercent(value)
}
function formatMs(value: number | null) {
  return formatMonitorMs(value)
}
function ttftCellState(state: HealthState | undefined, metric: { p50_ms: number | null; sample_count?: number }) {
  return ttftDisplayState(state, metric)
}
function latencyDetail(metric: {
  p50_ms: number | null
  p90_ms?: number | null
  p95_ms: number | null
  avg_ms?: number | null
}) {
  return formatLatencyPrivacy(metric.p50_ms, metric.p90_ms, metric.avg_ms, metric.p95_ms)
}
/** KPI secondary: AVG · P90 under the P50 primary value. */
function latencyKpiSecondary(metric: {
  p90_ms?: number | null
  p95_ms: number | null
  avg_ms?: number | null
}) {
  return formatLatencyKpiSecondary(metric.avg_ms, metric.p90_ms, metric.p95_ms)
}
function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    timeZone: 'Asia/Shanghai',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}
let lastRange: MonitorRange = filter.value.range
watch(
  filter,
  () => {
    syncQuery()
    const rangeChanged = filter.value.range !== lastRange
    lastRange = filter.value.range
    if (rangeChanged) void reload(true)
    else void reloadMetricsOnly(true)
  },
  { deep: true }
)
watch(healthMode, syncQuery)
onMounted(() => void reload(false))
onBeforeUnmount(() => {
  controller?.abort()
  if (autoRefreshTimer) window.clearInterval(autoRefreshTimer)
})
</script>
