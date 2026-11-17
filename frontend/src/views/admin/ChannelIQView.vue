<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <header class="relative overflow-hidden rounded-3xl bg-gradient-to-br from-indigo-600 via-violet-600 to-fuchsia-600 p-6 text-white shadow-lg sm:p-8">
        <div class="absolute -right-12 -top-16 h-48 w-48 rounded-full bg-white/10 blur-2xl" aria-hidden="true"></div>
        <div class="relative flex flex-wrap items-start justify-between gap-5">
          <div class="max-w-2xl">
            <div class="mb-3 inline-flex h-11 w-11 items-center justify-center rounded-2xl bg-white/15 ring-1 ring-white/20">
              <Icon name="sparkles" size="lg" />
            </div>
            <h1 class="text-2xl font-black tracking-tight sm:text-3xl">{{ t('admin.channelIQ.title') }}</h1>
            <p class="mt-2 text-sm leading-6 text-white/80">{{ t('admin.channelIQ.description') }}</p>
          </div>
          <button type="button" class="btn border-white/30 bg-white/15 text-white hover:bg-white/25" :disabled="loading" @click="load()">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-2">{{ t('common.refresh') }}</span>
          </button>
        </div>
      </header>

      <section class="card flex items-center justify-between gap-4 p-5 sm:p-6">
        <div>
          <h2 id="iq-visibility-label" class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.channelIQ.visibility.label') }}</h2>
          <p id="iq-visibility-hint" class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.channelIQ.visibility.hint') }}</p>
        </div>
        <Toggle :model-value="visible" :disabled="visibilitySaving || loading" aria-labelledby="iq-visibility-label" aria-describedby="iq-visibility-hint" @update:model-value="setVisibility" />
      </section>

      <section class="card p-5 sm:p-6">
        <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('admin.channelIQ.config.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.channelIQ.config.description') }}</p>
          </div>
          <div class="flex items-center gap-2 rounded-full bg-gray-50 px-3 py-1.5 text-xs text-gray-600 dark:bg-dark-900 dark:text-gray-300">
            <span class="h-2 w-2 rounded-full" :class="form.enabled ? 'bg-emerald-500' : 'bg-gray-400'"></span>
            {{ form.enabled ? t('admin.channelIQ.config.enabled') : t('admin.channelIQ.config.disabled') }}
          </div>
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
          <div class="lg:col-span-2">
            <label class="input-label">{{ t('admin.channelIQ.config.key') }}</label>
            <Select
              v-model="form.account_id"
              :options="accountOptions"
              :loading="accountsLoading"
              searchable
              :placeholder="t('admin.channelIQ.config.keyPlaceholder')"
            />
            <p class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">{{ t('admin.channelIQ.config.keyHint') }}</p>
          </div>

          <div>
            <label class="input-label">{{ t('admin.channelIQ.config.model') }}</label>
            <input v-model="form.model_id" class="input" type="text" :placeholder="t('admin.channelIQ.config.modelPlaceholder')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.channelIQ.config.reasoning') }}</label>
            <Select v-model="form.reasoning_effort" :options="reasoningOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.channelIQ.config.interval') }}</label>
            <Select v-model.number="form.interval_seconds" :options="intervalOptions" />
          </div>
          <div class="flex items-center justify-between rounded-xl border border-gray-200 px-4 dark:border-dark-700">
            <div>
              <div class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.channelIQ.config.enabledLabel') }}</div>
              <div class="mt-0.5 text-xs text-gray-400">{{ t('admin.channelIQ.config.enabledHint') }}</div>
            </div>
            <Toggle v-model="form.enabled" />
          </div>
          <div class="lg:col-span-2">
            <label class="input-label">{{ t('admin.channelIQ.config.note') }}</label>
            <input v-model="form.note" class="input" type="text" maxlength="200" :placeholder="t('admin.channelIQ.config.notePlaceholder')" />
          </div>
          <div class="lg:col-span-2">
            <label class="input-label">{{ t('admin.channelIQ.config.prompt') }}</label>
            <textarea v-model="form.prompt" rows="5" class="input resize-y" :placeholder="t('admin.channelIQ.config.promptPlaceholder')"></textarea>
            <p class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">{{ t('admin.channelIQ.config.promptHint') }}</p>
          </div>
        </div>

        <div class="mt-5 flex flex-wrap justify-end gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
          <button v-if="!form.enabled" type="button" class="btn btn-secondary" :disabled="loading || saving || testRunning || !form.account_id" @click="runNow">
            <Icon name="refresh" size="sm" :class="testRunning ? 'animate-spin' : ''" />
            <span class="ml-2">{{ testRunning ? t('admin.channelIQ.results.running') : t('admin.channelIQ.config.runNow') }}</span>
          </button>
          <button type="button" class="btn btn-primary" :disabled="loading || saving || testRunning || !form.account_id" @click="save">
            <Icon name="check" size="sm" />
            <span class="ml-2">{{ saving ? t('common.submitting') : t('common.save') }}</span>
          </button>
        </div>
      </section>

      <section>
        <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('admin.channelIQ.results.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.channelIQ.results.description') }}</p>
          </div>
          <span class="text-xs text-gray-400">{{ t('admin.channelIQ.results.count', { count: results.length }) }}</span>
        </div>

        <div v-if="results.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-2">
          <article v-for="result in results" :key="result.id" class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="absolute inset-x-0 top-0 h-1" :class="result.status === 'running' ? 'bg-amber-500' : result.status === 'success' ? 'bg-emerald-500' : 'bg-rose-500'"></div>
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="max-w-full break-words text-xs text-gray-400">{{ t('admin.channelIQ.results.startedAt') }} {{ formatTime(result.started_at) }} · {{ result.note || t('admin.channelIQ.results.noNote') }}</div>
                <div class="mt-2 flex flex-wrap gap-1.5 text-xs">
                  <span class="rounded-full bg-violet-50 px-2 py-1 font-medium text-violet-700 dark:bg-violet-900/25 dark:text-violet-300">{{ result.model_id || t('admin.channelIQ.results.autoModel') }}</span>
                  <span class="rounded-full bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ result.reasoning_effort || t('admin.channelIQ.results.autoReasoning') }}</span>
                </div>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="result.status === 'running' ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300' : result.status === 'success' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300' : 'bg-rose-50 text-rose-700 dark:bg-rose-900/25 dark:text-rose-300'">
                {{ result.status === 'running' ? t('admin.channelIQ.results.running') : result.status === 'success' ? t('admin.channelIQ.results.success') : t('admin.channelIQ.results.failed') }}
              </span>
            </div>
            <ChannelIQPreview v-if="result.status === 'success'" :response-text="result.response_text" :title="t('admin.channelIQ.results.animationTitle')" />
            <p v-else-if="result.status === 'running'" class="mt-4 rounded-xl bg-amber-50 px-3 py-2.5 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.channelIQ.results.runningDetail') }}</p>
            <p v-else class="mt-4 max-h-24 overflow-auto rounded-xl bg-rose-50 px-3 py-2.5 text-sm text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">{{ result.error_message || t('admin.channelIQ.results.empty') }}</p>
            <div v-if="result.evaluation" class="mt-4 rounded-xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-900/50">
              <div class="flex items-center justify-between gap-3 text-xs">
                <span class="font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.channelIQ.results.rubricTitle') }}</span>
                <span class="rounded-full px-2 py-1 font-semibold" :class="evaluationGradeClass(result)">
                  {{ evaluationGradeLabel(result) }} · {{ result.evaluation.score }}/4
                </span>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-2 text-xs">
                <div v-for="criterion in evaluationCriteria(result)" :key="criterion.key" class="flex items-center justify-between gap-2 rounded-lg bg-white px-2.5 py-2 dark:bg-dark-800">
                  <span class="text-gray-600 dark:text-gray-300">{{ criterion.label }}</span>
                  <span class="shrink-0 rounded-full px-1.5 py-0.5 font-semibold" :class="criterionStatusClass(criterion.status)">{{ criterionStatusLabel(criterion.status) }}</span>
                </div>
              </div>
            </div>
            <div v-if="result.status !== 'running'" class="mt-4 flex items-center justify-between text-xs text-gray-400">
              <span>{{ result.latency_ms }} ms</span>
              <span>{{ t('admin.channelIQ.results.finishedAt') }} {{ formatDate(result.finished_at) }}</span>
            </div>
          </article>
        </div>
        <div v-else class="card p-10 text-center">
          <Icon name="sparkles" size="lg" class="mx-auto text-violet-400" />
          <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.channelIQ.results.emptyState') }}</p>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { DASHBOARD_TIMEZONE } from '@/utils/dashboardTime'
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ChannelIQPreview from '@/components/common/ChannelIQPreview.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { AccountListItem } from '@/types'
import type { ChannelIQCriterionStatus, ChannelIQResult } from '@/api/admin/channelIQ'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const running = ref(false)
const visible = ref(true)
const visibilitySaving = ref(false)
let refreshTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let resultRevision = 0
let runRequestKey: string | undefined
const accountsLoading = ref(false)
const accounts = ref<AccountListItem[]>([])
const results = ref<ChannelIQResult[]>([])
const hasRunningResult = computed(() => results.value.some(result => result.status === 'running'))
const testRunning = computed(() => running.value || hasRunningResult.value)
const form = reactive({ account_id: 0, model_id: '', prompt: '', note: '', reasoning_effort: '', interval_seconds: 3600, enabled: false })

const accountOptions = computed(() => accounts.value.map(account => ({
  value: account.id,
  label: `${account.name || account.platform || `#${account.id}`} · ${account.platform || 'unknown'} · #${account.id}`
})))
const reasoningOptions = computed(() => [
  { value: '', label: t('admin.channelIQ.config.reasoningOptions.auto') },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
  { value: 'max', label: 'max' },
  { value: 'ultra', label: 'ultra' },
])
const intervalOptions = computed(() => [
  { value: 3600, label: t('admin.channelIQ.config.intervalOptions.hour') },
  { value: 5400, label: t('admin.channelIQ.config.intervalOptions.ninetyMinutes') },
  { value: 7200, label: t('admin.channelIQ.config.intervalOptions.twoHours') },
])
function applySnapshot(snapshot: Awaited<ReturnType<typeof adminAPI.channelIQ.get>>) {
  if (!visibilitySaving.value) visible.value = snapshot.visible !== false
  results.value = snapshot.results || []
  if (snapshot.config) {
    form.account_id = snapshot.config.account_id
    form.model_id = snapshot.config.model_id || ''
    form.prompt = snapshot.config.prompt || ''
    form.note = snapshot.config.note || ''
    form.reasoning_effort = snapshot.config.reasoning_effort || ''
    form.interval_seconds = snapshot.config.interval_seconds || 3600
    form.enabled = snapshot.config.enabled
  }
}

async function loadAccounts() {
  accountsLoading.value = true
  try {
    const page = await adminAPI.accounts.list(1, 1000, { status: 'active', lite: 'true' })
    accounts.value = (page.items || []).filter(account => account.status === 'active')
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.loadError')))
  } finally {
    accountsLoading.value = false
  }
}

async function load(silent = false) {
  if (!silent) loading.value = true
  const revision = resultRevision
  try {
    const snapshot = await adminAPI.channelIQ.get()
    // A read started before a task was accepted must not hide its running card.
    if (revision !== resultRevision || disposed) return
    if (silent) {
      // Background refresh updates results, not the account/model being edited.
      results.value = snapshot.results || []
      if (!visibilitySaving.value) visible.value = snapshot.visible !== false
    } else {
      applySnapshot(snapshot)
    }
  } catch (error: unknown) {
    if (!silent) appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.loadError')))
  } finally {
    if (!silent) loading.value = false
  }
}

async function save() {
  if (saving.value || testRunning.value || !form.account_id) return
  resultRevision++
  saving.value = true
  try {
    const snapshot = await adminAPI.channelIQ.save({ ...form })
    applySnapshot(snapshot)
    appStore.showSuccess(t('admin.channelIQ.saveSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.saveError')))
  } finally {
    saving.value = false
  }
}

async function setVisibility(nextVisible: boolean) {
  if (visibilitySaving.value || loading.value) return
  visibilitySaving.value = true
  try {
    const result = await adminAPI.channelIQ.setVisibility(nextVisible)
    visible.value = result.visible
    if (appStore.cachedPublicSettings) appStore.cachedPublicSettings.channel_iq_enabled = result.visible
    appStore.showSuccess(t(result.visible ? 'admin.channelIQ.visibility.shown' : 'admin.channelIQ.visibility.hidden'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.visibility.saveError')))
  } finally {
    visibilitySaving.value = false
  }
}

async function runNow() {
  if (testRunning.value || saving.value || !form.account_id) return
  running.value = true
  resultRevision++
  try {
    // The server probes the persisted plan. Save the current selection first so
    // switching away from an OAuth account actually tests the new API key.
    const snapshot = await adminAPI.channelIQ.save({ ...form })
    applySnapshot(snapshot)
    runRequestKey ??= crypto.randomUUID()
    const result = await adminAPI.channelIQ.run(runRequestKey)
    resultRevision++
    results.value = [result, ...results.value.filter(item => item.id !== result.id)].slice(0, 6)
    runRequestKey = undefined
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.runError')))
  } finally {
    running.value = false
    scheduleRefresh()
  }
}

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  if (disposed) return
  refreshTimer = setTimeout(async () => {
    if (!saving.value && !running.value) await load(true)
    scheduleRefresh()
  }, hasRunningResult.value ? 5000 : 30000)
}

function evaluationGradeLabel(result: ChannelIQResult): string {
  const evaluation = result.evaluation
  if (!evaluation?.graded) return t('admin.channelIQ.results.grades.ungraded')
  return t(`admin.channelIQ.results.grades.${evaluation.grade}`)
}

function evaluationGradeClass(result: ChannelIQResult): string {
  const evaluation = result.evaluation
  if (!evaluation?.graded) return 'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (evaluation.grade === 'baseline') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (evaluation.grade === 'mild') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (evaluation.grade === 'moderate') return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300'
  return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
}

function evaluationCriteria(result: ChannelIQResult): Array<{ key: string; label: string; status: ChannelIQCriterionStatus }> {
  const criteria = result.evaluation?.criteria
  if (!criteria) return []
  return [
    { key: 'wheels', label: t('admin.channelIQ.results.criteria.wheels'), status: criteria.wheels.status },
    { key: 'leg_pedal', label: t('admin.channelIQ.results.criteria.legPedal'), status: criteria.leg_pedal.status },
    { key: 'view_consistent', label: t('admin.channelIQ.results.criteria.viewConsistent'), status: criteria.view_consistent.status },
    { key: 'pelican_beak', label: t('admin.channelIQ.results.criteria.pelicanBeak'), status: criteria.pelican_beak.status },
  ]
}

function criterionStatusLabel(status: ChannelIQCriterionStatus): string {
  return t(`admin.channelIQ.results.criterionStatus.${status}`)
}

function criterionStatusClass(status: ChannelIQCriterionStatus): string {
  if (status === 'pass') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'partial') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (status === 'fail') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  return 'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString(undefined, { timeZone: DASHBOARD_TIMEZONE }) : '—'
}

function formatDate(value?: string): string {
  return value ? new Date(value).toLocaleTimeString([], { timeZone: DASHBOARD_TIMEZONE, hour: '2-digit', minute: '2-digit' }) : '—'
}

onMounted(async () => {
  await Promise.all([loadAccounts(), load()])
  scheduleRefresh()
})

onUnmounted(() => {
  disposed = true
  if (refreshTimer) clearTimeout(refreshTimer)
})
</script>
