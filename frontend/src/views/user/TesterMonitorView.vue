<template>
  <AppLayout :header-title="t('tester.title')" :header-subtitle="t('tester.description')">
    <div class="space-y-5" :aria-busy="loading">
      <section class="rounded-2xl border border-primary-200 bg-gradient-to-br from-primary-50 via-white to-emerald-50 p-5 dark:border-primary-800/60 dark:from-dark-800 dark:via-dark-800 dark:to-primary-950/30">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <span class="badge badge-primary">{{ t('tester.role') }}</span>
            <p class="mt-3 text-sm text-gray-600 dark:text-dark-300">{{ t('tester.manual') }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('tester.updated') }}：{{ dateTime(snapshot?.generated_at) }} · {{ t('tester.resetAt') }}：{{ snapshot?.visible_after ? dateTime(snapshot.visible_after) : t('tester.notReset') }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary" data-test="export-tester-report" :disabled="loading || exporting || !snapshot" @click="exportReport">
              <Icon :name="exporting ? 'refresh' : 'download'" size="sm" class="mr-1.5" :class="exporting && 'animate-spin'" />
              {{ t(exporting ? 'tester.exportingData' : 'tester.exportData') }}
            </button>
            <TesterHistoryResetButton :disabled="loading" @reset="afterReset" />
            <button class="btn btn-primary" :disabled="loading" @click="load">{{ t('tester.refresh') }}</button>
          </div>
        </div>
        <form class="mt-5 flex flex-wrap items-center gap-3" @submit.prevent="applyFilters">
          <Select v-model="period" class="w-40" :options="periodOptions" :searchable="false" @change="applyFilters" />
          <Select v-model="keyId" class="w-full sm:w-56" :options="keyOptions" searchable @change="applyFilters" />
          <input v-model="model" class="input w-full sm:!w-64" :placeholder="t('tester.model')" :aria-label="t('tester.model')" maxlength="200" />
          <button type="submit" class="btn btn-secondary" :disabled="loading">{{ t('tester.apply') }}</button>
          <button v-if="model || groupId || keyId" type="button" class="btn btn-secondary" :disabled="loading" @click="clearFilters">{{ t('tester.clearFilters') }}</button>
          <button v-if="hasMoreKeys" type="button" class="text-sm text-primary-700 underline dark:text-primary-300" @click="loadMoreKeys">{{ t('tester.moreKeys') }}</button>
          <button v-if="groupId" type="button" class="badge badge-primary" @click="groupId = undefined; applyFilters()">{{ t('tester.group') }}：{{ groupName || `#${groupId}` }} ×</button>
        </form>
      </section>

      <p v-if="loadFailed || keysFailed" role="alert" class="rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200">{{ t('tester.loadFailed') }}</p>

      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
        <article v-for="card in cards" :key="card.label" class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm transition-colors hover:border-primary-300 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-600" :title="card.hint">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-300">{{ card.label }}</p>
          <p class="mt-3 break-words text-2xl font-semibold tabular-nums tracking-tight text-gray-900 dark:text-white">{{ card.value }}</p>
          <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ card.secondary }}</p>
        </article>
      </div>
      <p class="text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('tester.timingHint') }} {{ t('tester.metricsHint') }}</p>

      <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('tester.modelTitle') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('tester.modelHint') }}</p>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-300">
              <tr><th class="px-5 py-3">{{ t('tester.modelColumn') }}</th><th class="px-4 py-3">{{ t('tester.requestCount') }}</th><th class="px-4 py-3">{{ t('tester.spend') }}</th><th class="px-4 py-3" :title="t('tester.cacheHint')">{{ t('tester.cache') }}</th><th class="px-4 py-3">{{ t('tester.ttft') }} P50 / P95</th><th class="px-4 py-3">{{ t('tester.latency') }} P50 / P95</th><th class="px-4 py-3">{{ t('tester.speed') }}</th><th class="px-4 py-3" :title="t('tester.unitHint')">{{ t('tester.unitCost') }}</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in snapshot?.models || []" :key="`${row.model}:${row.group_id}`" class="transition-colors hover:bg-primary-50/70 dark:hover:bg-primary-900/15">
                <td class="px-5 py-4"><button class="text-left font-medium text-gray-900 hover:text-primary-700 dark:text-white dark:hover:text-primary-300" @click="selectModel(row)">{{ row.model }}<span class="mt-1 block text-xs font-normal text-gray-500 dark:text-dark-400">{{ row.group_name || t('tester.ungrouped') }}</span></button></td>
                <td class="px-4 py-4 tabular-nums">{{ row.requests }}</td>
                <td class="px-4 py-4 tabular-nums">{{ testerMoney(row.actual_cost) }}</td>
                <td class="min-w-32 px-4 py-4"><span class="font-medium text-primary-700 dark:text-primary-300">{{ row.cache_hit_rate.toFixed(1) }}%</span><div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full bg-primary-500" :style="{ width: `${Math.max(0, Math.min(100, row.cache_hit_rate))}%` }" /></div></td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums">{{ testerTime(row.first_token_p50_ms) }} / {{ testerTime(row.first_token_p95_ms) }}</td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums">{{ testerTime(row.duration_p50_ms) }} / {{ testerTime(row.duration_p95_ms) }}</td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums">{{ speed(row.output_tokens_per_second) }}</td>
                <td class="px-4 py-4 tabular-nums" :title="t('tester.unitHint')">{{ testerMoney(row.cost_per_million_tokens) }}</td>
              </tr>
              <tr v-if="!snapshot?.models.length"><td colspan="8" class="px-5 py-12 text-center text-gray-500 dark:text-dark-400">{{ loading ? t('common.loading') : t('tester.empty') }}</td></tr>
            </tbody>
          </table>
        </div>
        <p v-if="snapshot?.models_truncated" class="px-5 py-3 text-xs text-amber-700 dark:text-amber-300">{{ t('tester.truncated') }}</p>
      </section>

      <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center gap-2 border-b border-gray-100 p-3 dark:border-dark-700">
          <button v-for="tab in (['samples', 'errors'] as const)" :key="tab" class="rounded-xl px-4 py-2 text-sm font-medium transition-colors" :class="activeTab === tab ? 'bg-primary-100 text-primary-800 dark:bg-primary-900/40 dark:text-primary-200' : 'text-gray-500 hover:bg-gray-100 dark:text-dark-300 dark:hover:bg-dark-700'" @click="changeTab(tab)">{{ t(`tester.${tab}`) }}</button>
        </div>
        <div class="overflow-x-auto">
          <table v-if="activeTab === 'samples'" class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-300"><tr><th class="px-5 py-3">{{ t('tester.time') }}</th><th class="px-4 py-3">{{ t('tester.modelColumn') }}</th><th class="px-4 py-3">{{ t('tester.key') }}</th><th class="px-4 py-3">{{ t('tester.spend') }}</th><th class="px-4 py-3">{{ t('tester.tokens') }}</th><th class="px-4 py-3">{{ t('tester.cache') }}</th><th class="px-4 py-3">{{ t('tester.ttft') }} / {{ t('tester.latency') }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in requests" :key="row.id" class="transition-colors hover:bg-primary-50/70 dark:hover:bg-primary-900/15">
                <td class="whitespace-nowrap px-5 py-4 text-xs text-gray-500 dark:text-dark-300">{{ dateTime(row.created_at) }}</td>
                <td class="px-4 py-4"><button class="text-left font-medium text-primary-700 hover:underline dark:text-primary-300" @click="selectedRecord = row">{{ row.model }}<span class="mt-1 block text-xs font-normal text-gray-500 dark:text-dark-400">{{ row.group?.name || t('tester.ungrouped') }}</span></button></td>
                <td class="max-w-36 truncate px-4 py-4" :title="row.api_key?.name">{{ row.api_key?.name || `#${row.api_key_id}` }}</td>
                <td class="px-4 py-4 font-medium tabular-nums">{{ testerMoney(row.actual_cost) }}</td>
                <td class="whitespace-nowrap px-4 py-4 text-xs tabular-nums">{{ testerTokens(row.input_tokens) }} / {{ testerTokens(row.output_tokens) }} / {{ testerTokens(row.cache_read_tokens) }}</td>
                <td class="px-4 py-4 tabular-nums">{{ testerCacheRate(row).toFixed(1) }}%</td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums">{{ testerTime(row.first_token_ms) }} / {{ testerTime(row.duration_ms) }}</td>
              </tr>
              <tr v-if="!requests.length"><td colspan="7" class="px-5 py-10 text-center text-gray-500 dark:text-dark-400">{{ loading ? t('common.loading') : t('tester.empty') }}</td></tr>
            </tbody>
          </table>
          <table v-else class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-300"><tr><th class="px-5 py-3">{{ t('tester.time') }}</th><th class="px-4 py-3">{{ t('tester.modelColumn') }}</th><th class="px-4 py-3">{{ t('tester.key') }}</th><th class="px-4 py-3">{{ t('tester.status') }}</th><th class="px-4 py-3">{{ t('tester.message') }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in errors" :key="row.id"><td class="whitespace-nowrap px-5 py-4 text-xs">{{ dateTime(row.created_at) }}</td><td class="px-4 py-4">{{ row.model }}<span class="block text-xs text-gray-500 dark:text-dark-400">{{ row.group_name }}</span></td><td class="px-4 py-4">{{ row.key_name }}</td><td class="px-4 py-4"><span class="rounded-lg bg-rose-50 px-2 py-1 text-rose-700 dark:bg-rose-950/30 dark:text-rose-300">{{ row.status_code }}</span></td><td class="max-w-md px-4 py-4 text-xs leading-6 text-gray-600 dark:text-dark-300">{{ row.message }}</td></tr><tr v-if="!errors.length"><td colspan="5" class="px-5 py-10 text-center text-gray-500 dark:text-dark-400">{{ loading ? t('common.loading') : t('tester.empty') }}</td></tr></tbody>
          </table>
        </div>
        <div class="flex items-center justify-end gap-3 border-t border-gray-100 px-4 py-3 text-sm dark:border-dark-700"><button class="btn btn-secondary" :disabled="loading || page <= 1" @click="changePage(-1)">{{ t('tester.previous') }}</button><span class="text-gray-500 dark:text-dark-300">{{ t('tester.page', { page }) }}</span><button class="btn btn-secondary" :disabled="loading || !hasNext" @click="changePage(1)">{{ t('tester.next') }}</button></div>
      </section>
      <p class="text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('tester.sessionHint') }} {{ t('tester.unitHint') }}</p>
    </div>

    <BaseDialog :show="!!selectedRecord" :title="t('tester.detail')" width="wide" @close="selectedRecord = null">
      <div v-if="selectedRecord" class="space-y-5">
        <div><p class="text-lg font-semibold text-gray-900 dark:text-white">{{ selectedRecord.model }}</p><p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ selectedRecord.group?.name }} · {{ dateTime(selectedRecord.created_at) }}</p></div>
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('tester.costHint') }}</p>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3"><div v-for="entry in detailEntries" :key="entry.label" class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/40"><p class="text-xs text-gray-500 dark:text-dark-400">{{ entry.label }}</p><p class="mt-2 break-words font-medium tabular-nums text-gray-900 dark:text-white">{{ entry.value }}</p></div></div>
        <p class="break-all rounded-xl bg-gray-50 p-3 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-300">{{ t('tester.requestId') }}：{{ selectedRecord.request_id }}</p>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TesterHistoryResetButton from '@/components/user/TesterHistoryResetButton.vue'
import { useAppStore } from '@/stores/app'
import { apiClient } from '@/api/client'
import { keysAPI, usageAPI } from '@/api'
import { getTesterSnapshot, type TesterMetric, type TesterSnapshot } from '@/api/tester'
import { testerCacheRate, testerMoney, testerRange, testerTime, testerTokens, type TesterPeriod } from '@/utils/testerMetrics'
import type { ApiKey, UsageLog, UserErrorRequest, PaginatedResponse } from '@/types'
import { saveAs } from 'file-saver'

const { t } = useI18n()
const appStore = useAppStore()
const snapshot = ref<TesterSnapshot | null>(null)
const period = ref<TesterPeriod>('today')
const keyId = ref<number | null>(null)
const model = ref('')
const groupId = ref<number | undefined>()
const groupName = ref('')
const keys = ref<ApiKey[]>([])
const hasMoreKeys = ref(false)
let keyPage = 0
let keysLoading = false
const loading = ref(false)
const loadFailed = ref(false)
const keysFailed = ref(false)
const keyController = new AbortController()
const page = ref(1)
const hasNext = ref(false)
const activeTab = ref<'samples' | 'errors'>('samples')
const requests = ref<UsageLog[]>([])
const errors = ref<UserErrorRequest[]>([])
const selectedRecord = ref<UsageLog | null>(null)
const exporting = ref(false)
const appliedFilters = ref<{ period: TesterPeriod; keyId: number | null; groupId?: number; groupName: string; model: string }>({ period: 'today', keyId: null, groupName: '', model: '' })
let exportController: AbortController | null = null
let controller: AbortController | undefined
let seq = 0
const periodOptions = computed(() => (['today', 'week', 'month', 'session'] as const).map(value => ({ value, label: t(`tester.${value}`) })))
const keyOptions = computed(() => [{ value: null, label: t('tester.allKeys') }, ...keys.value.map(key => ({ value: key.id, label: key.name || `#${key.id}` }))])
const speed = (value?: number | null) => value != null ? `${value.toFixed(1)} tok/s` : '—'
const dateTime = (value?: string | null) => value ? new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }) : '—'
const exportDateTime = (value: string) => `${new Intl.DateTimeFormat('sv-SE', { timeZone:'Asia/Shanghai', year:'numeric', month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', second:'2-digit', hourCycle:'h23' }).format(new Date(value))} +08:00`
const cards = computed(() => {
  const s = snapshot.value?.summary
  return [
    { label:t('tester.billed'), value:testerMoney(s?.actual_cost), secondary:`${t('tester.nominal')} ${testerMoney(s?.total_cost)}`, hint:t('tester.unitHint') },
    { label:t('tester.cache'), value:s ? `${s.cache_hit_rate.toFixed(1)}%` : '—', secondary:s ? testerTokens(s.cache_read_tokens) : '—', hint:t('tester.cacheHint') },
    { label:`${t('tester.ttft')} P50`, value:testerTime(s?.first_token_p50_ms), secondary:`P95 ${testerTime(s?.first_token_p95_ms)}`, hint:t('tester.timingHint') },
    { label:`${t('tester.latency')} P50`, value:testerTime(s?.duration_p50_ms), secondary:`P95 ${testerTime(s?.duration_p95_ms)}`, hint:t('tester.timingHint') },
    { label:t('tester.speed'), value:speed(s?.output_tokens_per_second), secondary:s ? t('tester.streamSamples', { count:s.output_samples }) : '—', hint:t('tester.timingHint') },
    { label:t('tester.requests'), value:s ? `${s.requests}` : '—', secondary:`${t('tester.failures')} ${snapshot.value?.error_records ?? '—'}`, hint:t('tester.metricsHint') },
  ]
})
const detailEntries = computed(() => {
  const r = selectedRecord.value
  if (!r) return []
  return [
    {label:t('tester.billed'),value:testerMoney(r.actual_cost)}, {label:t('tester.nominal'),value:testerMoney(r.total_cost)},
    {label:t('tester.multiplier'),value:`${r.rate_multiplier}×`}, {label:t('tester.inputCost'),value:testerMoney(r.input_cost)},
    {label:t('tester.outputCost'),value:testerMoney(r.output_cost)}, {label:t('tester.cacheReadCost'),value:testerMoney(r.cache_read_cost)},
    {label:t('tester.cacheWriteCost'),value:testerMoney(r.cache_creation_cost)}, {label:t('tester.ttft'),value:testerTime(r.first_token_ms)},
    {label:t('tester.latency'),value:testerTime(r.duration_ms)}, {label:t('tester.cache'),value:`${testerCacheRate(r).toFixed(1)}%`},
    {label:t('tester.tokens'),value:`${testerTokens(r.input_tokens)} / ${testerTokens(r.output_tokens)} / ${testerTokens(r.cache_read_tokens)}`},
    {label:t('tester.billingMode'),value:r.billing_mode || 'token'},
    {label:t('tester.cacheWriteTokens'),value:testerTokens(r.cache_creation_tokens)},
  ]
})
async function loadMoreKeys() {
  if (keysLoading) return
  keysLoading = true
  try {
    const result = await keysAPI.list(keyPage + 1, 100, undefined, { signal: keyController.signal })
    keyPage++
    keys.value.push(...result.items)
    hasMoreKeys.value = result.items.length === 100 && keys.value.length < result.total
    keysFailed.value = false
  } catch { if (!keyController.signal.aborted) keysFailed.value = true }
  finally { keysLoading = false }
}
async function load() {
  if (keysFailed.value) void loadMoreKeys()
  controller?.abort()
  controller = new AbortController()
  const signal = controller.signal
  const current = ++seq
  const selectedPeriod = period.value
  const selectedKeyId = keyId.value
  const selectedGroupId = groupId.value
  const selectedModel = model.value.trim()
  loading.value = true
  const params = { ...testerRange(selectedPeriod, new Date(), snapshot.value?.visible_after), api_key_id:selectedKeyId || undefined, group_id:selectedGroupId, model:selectedModel || undefined }
  try {
    const [stats, usage, failed] = await Promise.all([
      getTesterSnapshot(params, signal),
      activeTab.value === 'samples' ? usageAPI.query({ ...params, page:page.value, page_size:20 }, { signal }) : Promise.resolve(null),
      activeTab.value === 'errors' ? apiClient.get<PaginatedResponse<UserErrorRequest>>('/usage/errors', { params:{ ...params, page:page.value, page_size:20 }, signal }) : Promise.resolve(null),
    ])
    if (signal.aborted || current !== seq) return
    snapshot.value = stats
    appliedFilters.value = { period:selectedPeriod, keyId:selectedKeyId, groupId:selectedGroupId, groupName:groupName.value, model:selectedModel }
    requests.value = usage?.items || []
    errors.value = failed?.data.items || []
    hasNext.value = page.value * 20 < (usage?.total ?? failed?.data.total ?? 0)
    loadFailed.value = false
  } catch { if (!signal.aborted && current === seq) loadFailed.value = true }
  finally { if (current === seq) loading.value = false }
}
async function exportReport() {
  const current = snapshot.value
  if (!current || exporting.value) return
  exporting.value = true
  const controller = new AbortController()
  exportController = controller
  const filters = { ...appliedFilters.value }
  try {
    const XLSX = await import('xlsx')
    const pageSize = 1000
    const requestRows: UsageLog[] = []
    const requestParams = {
      start_time:current.start_time,
      end_time:current.end_time,
      timezone:'Asia/Shanghai',
      api_key_id:filters.keyId || undefined,
      group_id:filters.groupId,
      model:filters.model || undefined,
      page_size:pageSize,
      sort_by:'created_at' as const,
      sort_order:'asc' as const,
    }
    if (Date.parse(current.end_time) > Date.parse(current.start_time)) {
      for (let page = 1, total = 0; ; page++) {
        const result = await usageAPI.query({ ...requestParams, page }, { signal:controller.signal })
        requestRows.push(...result.items)
        total = result.total
        if (requestRows.length >= total || result.items.length < pageSize) break
      }
    }
    if (controller.signal.aborted) return
    const summary = current.summary
    const keyName = filters.keyId
      ? keys.value.find(key => key.id === filters.keyId)?.name || `#${filters.keyId}`
      : t('tester.allKeys')
    const overview = [
      [t('tester.exportField'), t('tester.exportValue')],
      [t('tester.exportedAt'), exportDateTime(new Date().toISOString())],
      [t('tester.exportPeriod'), t(`tester.${filters.period}`)],
      [t('tester.exportStart'), exportDateTime(current.start_time)],
      [t('tester.exportEnd'), exportDateTime(current.end_time)],
      [t('tester.exportKey'), keyName],
      [t('tester.exportGroup'), filters.groupId ? filters.groupName : t('tester.allGroups')],
      [t('tester.exportModel'), filters.model || t('tester.allModels')],
      [t('tester.modelCount'), current.models.length],
      [t('tester.exportRequestRows'), requestRows.length],
      [t('tester.requests'), summary.requests],
      [t('tester.failures'), current.error_records],
      [t('tester.exportInputTokens'), summary.input_tokens],
      [t('tester.exportOutputTokens'), summary.output_tokens],
      [t('tester.exportCacheReadTokens'), summary.cache_read_tokens],
      [t('tester.exportCacheCreationTokens'), summary.cache_creation_tokens],
      [t('tester.nominal'), summary.total_cost],
      [t('tester.billed'), summary.actual_cost],
      [t('tester.cache'), summary.cache_hit_rate],
      [`${t('tester.ttft')} P50 (ms)`, summary.first_token_p50_ms],
      [`${t('tester.ttft')} P95 (ms)`, summary.first_token_p95_ms],
      [`${t('tester.latency')} P50 (ms)`, summary.duration_p50_ms],
      [`${t('tester.latency')} P95 (ms)`, summary.duration_p95_ms],
      [t('tester.speed'), summary.output_tokens_per_second],
      [t('tester.unitCost'), summary.cost_per_million_tokens],
      [t('tester.exportDataLimit'), current.models_truncated ? t('tester.exportTruncated') : t('tester.exportComplete')],
    ]
    const modelRows: (string | number | null)[][] = [[
      t('tester.modelColumn'), t('tester.group'), t('tester.groupId'), t('tester.requestCount'),
      t('tester.exportInputTokens'), t('tester.exportOutputTokens'),
      t('tester.exportCacheReadTokens'), t('tester.exportCacheCreationTokens'),
      t('tester.exportNominalCost'), t('tester.exportActualCost'), t('tester.exportCacheHitRate'),
      `${t('tester.ttft')} P50 (ms)`, `${t('tester.ttft')} P95 (ms)`,
      `${t('tester.latency')} P50 (ms)`, `${t('tester.latency')} P95 (ms)`,
      t('tester.speed'), t('tester.unitCost'),
    ], ...current.models.map(row => [
      row.model, row.group_name || t('tester.ungrouped'), row.group_id, row.requests,
      row.input_tokens, row.output_tokens, row.cache_read_tokens, row.cache_creation_tokens,
      row.total_cost, row.actual_cost, row.cache_hit_rate,
      row.first_token_p50_ms, row.first_token_p95_ms,
      row.duration_p50_ms, row.duration_p95_ms,
      row.output_tokens_per_second, row.cost_per_million_tokens,
    ])]
    const requestTable: (string | number | boolean | null)[][] = [[
      t('tester.requestId'), t('tester.time'), t('tester.modelColumn'), t('tester.group'), t('tester.groupId'), t('tester.key'),
      t('tester.exportInputTokens'), t('tester.exportOutputTokens'), t('tester.exportCacheReadTokens'), t('tester.exportCacheCreationTokens'),
      t('tester.exportCacheHitRate'), `${t('tester.ttft')} (ms)`, `${t('tester.latency')} (ms)`, t('tester.exportNominalCost'), t('tester.exportActualCost'),
      t('tester.billingMode'), t('tester.multiplier'), t('tester.stream'),
    ], ...requestRows.map(row => [
      row.request_id, exportDateTime(row.created_at), row.model, row.group?.name || t('tester.ungrouped'), row.group_id || 0,
      row.api_key?.name || `#${row.api_key_id}`, row.input_tokens, row.output_tokens, row.cache_read_tokens, row.cache_creation_tokens,
      testerCacheRate(row), row.first_token_ms, row.duration_ms, row.total_cost, row.actual_cost,
      row.billing_mode || 'token', row.rate_multiplier, row.stream ? t('common.yes') : t('common.no'),
    ])]
    const overviewSheet = XLSX.utils.aoa_to_sheet(overview)
    overviewSheet['!cols'] = [{ wch: 28 }, { wch: 38 }]
    const modelsSheet = XLSX.utils.aoa_to_sheet(modelRows)
    modelsSheet['!cols'] = [{ wch: 28 }, { wch: 22 }, { wch: 14 }, ...Array.from({ length: 14 }, () => ({ wch: 18 }))]
    modelsSheet['!autofilter'] = { ref: `A1:Q${Math.max(1, modelRows.length)}` }
    const requestsSheet = XLSX.utils.aoa_to_sheet(requestTable)
    requestsSheet['!cols'] = [{ wch: 28 }, { wch: 22 }, { wch: 28 }, { wch: 22 }, { wch: 14 }, { wch: 24 }, ...Array.from({ length: 12 }, () => ({ wch: 18 }))]
    requestsSheet['!autofilter'] = { ref: `A1:R${Math.max(1, requestTable.length)}` }
    const workbook = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(workbook, overviewSheet, t('tester.exportOverview').slice(0, 31))
    XLSX.utils.book_append_sheet(workbook, modelsSheet, t('tester.exportModels').slice(0, 31))
    XLSX.utils.book_append_sheet(workbook, requestsSheet, t('tester.exportRequests').slice(0, 31))
    workbook.Props = { Title:t('tester.exportTitle'), Subject:t('tester.exportSubject'), CreatedDate:new Date() }
    const bytes = XLSX.write(workbook, { bookType:'xlsx', type:'array' })
    const day = exportDateTime(new Date().toISOString()).slice(0, 10)
    saveAs(new Blob([bytes], { type:'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), `tester-model-report_${day}.xlsx`)
    appStore.showSuccess(t('tester.exportSuccess'))
  } catch {
    if (!controller.signal.aborted) appStore.showError(t('tester.exportFailed'))
  } finally {
    if (exportController === controller) {
      exportController = null
      exporting.value = false
    }
  }
}
function applyFilters() { page.value = 1; selectedRecord.value = null; void load() }
function clearFilters() { keyId.value = null; model.value = ''; groupId.value = undefined; applyFilters() }
function selectModel(row: TesterMetric) { model.value = row.model; groupId.value = row.group_id || undefined; groupName.value = row.group_name; applyFilters() }
function changeTab(tab: 'samples' | 'errors') { activeTab.value = tab; applyFilters() }
function changePage(delta: number) { page.value = Math.max(1, page.value + delta); void load() }
function afterReset(visibleAfter: string) {
  controller?.abort(); seq++
  selectedRecord.value = null
  if (snapshot.value) snapshot.value.visible_after = visibleAfter
  page.value = 1
  void load()
}
onMounted(() => { void load(); void loadMoreKeys() })
onUnmounted(() => { controller?.abort(); exportController?.abort(); keyController.abort(); seq++ })
</script>
