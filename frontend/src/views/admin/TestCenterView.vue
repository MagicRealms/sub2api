<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
                <Icon name="chart" size="sm" />
              </span>
              {{ t('admin.testCenter.title') }}
            </h1>
            <p class="page-description mt-1.5 max-w-3xl text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.testCenter.description') }}
            </p>
          </div>
          <span class="inline-flex items-center rounded-full bg-amber-50 px-3 py-1.5 text-xs font-semibold text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
            {{ t('admin.testCenter.directCallBadge') }}
          </span>
        </div>
        <div class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/70 px-4 py-3 text-xs leading-5 text-amber-800 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200">
          {{ t('admin.testCenter.directCallHint') }}
        </div>
        <div class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="tabs inline-flex w-full max-w-xl" role="tablist" :aria-label="t('admin.testCenter.tabsLabel')">
            <button
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="testTab === 'qa' ? 'tab-active' : ''"
              :aria-selected="testTab === 'qa'"
              @click="testTab = 'qa'"
            >
              {{ t('admin.testCenter.qaTab') }}
            </button>
            <button
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="testTab === 'image' ? 'tab-active' : ''"
              :aria-selected="testTab === 'image'"
              @click="testTab = 'image'"
            >
              {{ t('admin.testCenter.imageTab') }}
            </button>
          </div>
        </div>
      </header>

      <div class="grid gap-6 xl:grid-cols-[minmax(320px,0.9fr)_minmax(0,1.4fr)]">
        <section class="space-y-6">
          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
            <div class="flex items-start justify-between gap-3">
              <div>
                <h2 class="text-base font-bold text-gray-900 dark:text-white">{{ t('admin.testCenter.accountTitle') }}</h2>
                <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.testCenter.accountHint') }}</p>
              </div>
              <button
                type="button"
                class="btn btn-secondary btn-sm shrink-0"
                :disabled="loadingAccounts || isRunning"
                @click="loadAccounts"
              >
                <Icon name="refresh" size="sm" :class="loadingAccounts ? 'animate-spin' : ''" />
                {{ t('admin.testCenter.refresh') }}
              </button>
            </div>

            <div v-if="loadError" class="mt-4 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300">
              {{ loadError }}
            </div>

            <div v-if="loadingAccounts" class="mt-4 space-y-2">
              <div v-for="index in 3" :key="index" class="h-14 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700" />
            </div>
            <div v-else-if="accounts.length === 0" class="mt-4 rounded-xl border border-dashed border-gray-200 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              {{ t('admin.testCenter.noAccounts') }}
            </div>
            <template v-else>
              <div class="mt-4 flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                <span>{{ t('admin.testCenter.accountCount', { count: accounts.length }) }}</span>
                <div class="flex gap-2">
                  <button type="button" class="text-primary-600 hover:underline dark:text-primary-400" :disabled="isRunning" @click="selectAllAccounts">
                    {{ t('admin.testCenter.selectAll') }}
                  </button>
                  <button type="button" class="text-gray-500 hover:underline dark:text-gray-400" :disabled="isRunning" @click="clearAccounts">
                    {{ t('admin.testCenter.clear') }}
                  </button>
                </div>
              </div>
              <div class="mt-2 max-h-[330px] space-y-2 overflow-y-auto pr-1">
                <label
                  v-for="account in accounts"
                  :key="account.id"
                  class="flex cursor-pointer items-center gap-3 rounded-xl border px-3 py-3 transition-colors"
                  :class="selectedAccountIds.includes(account.id) ? 'border-primary-300 bg-primary-50/60 dark:border-primary-700 dark:bg-primary-900/20' : 'border-gray-200 hover:border-primary-200 dark:border-dark-600 dark:hover:border-primary-800'"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700"
                    :checked="selectedAccountIds.includes(account.id)"
                    :disabled="isRunning"
                    @change="toggleAccount(account.id)"
                  />
                  <span class="min-w-0 flex-1">
                    <span class="block truncate text-sm font-semibold text-gray-900 dark:text-white">{{ account.name }}</span>
                    <span class="mt-0.5 flex items-center gap-2 text-[11px] text-gray-500 dark:text-gray-400">
                      <span class="rounded bg-gray-100 px-1.5 py-0.5 uppercase dark:bg-dark-700">{{ account.platform }}</span>
                      <span>{{ account.type }}</span>
                    </span>
                  </span>
                  <span class="h-2 w-2 rounded-full bg-emerald-500" :title="t('admin.testCenter.active')" />
                </label>
              </div>
            </template>
          </div>

          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
            <div class="space-y-4">
              <div>
                <label class="mb-1.5 block text-sm font-semibold text-gray-800 dark:text-gray-200" for="test-center-model">
                  {{ t('admin.testCenter.model') }}
                </label>
                <input
                  id="test-center-model"
                  v-model="selectedModelId"
                  list="test-center-model-options"
                  class="input w-full"
                  :placeholder="loadingModels ? t('admin.testCenter.loadingModels') : t('admin.testCenter.modelPlaceholder')"
                  :disabled="isRunning || selectedAccountIds.length === 0"
                />
                <datalist id="test-center-model-options">
                  <option v-for="model in modelOptions" :key="model.id" :value="model.id">{{ model.display_name || model.id }}</option>
                </datalist>
                <p class="mt-1.5 text-xs leading-5 text-gray-500 dark:text-gray-400">
                  {{ testTab === 'image' ? t('admin.testCenter.imageModelHint') : t('admin.testCenter.qaModelHint') }}
                </p>
              </div>

              <div>
                <label class="mb-1.5 block text-sm font-semibold text-gray-800 dark:text-gray-200" for="test-center-prompt">
                  {{ t('admin.testCenter.prompt') }}
                </label>
                <textarea
                  id="test-center-prompt"
                  v-model="prompt"
                  class="input min-h-[88px] w-full resize-y"
                  :placeholder="testTab === 'image' ? t('admin.testCenter.imagePromptPlaceholder') : t('admin.testCenter.qaPromptPlaceholder')"
                  :disabled="isRunning"
                  rows="3"
                />
              </div>

              <div class="grid gap-3 sm:grid-cols-2">
                <div>
                  <label class="mb-1.5 block text-sm font-semibold text-gray-800 dark:text-gray-200" for="test-center-rounds">
                    {{ t('admin.testCenter.rounds') }}
                  </label>
                  <input id="test-center-rounds" v-model.number="rounds" type="number" min="1" :max="maxRounds" class="input w-full" :disabled="isRunning" />
                  <p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.testCenter.roundsHint', { max: maxRounds }) }}</p>
                </div>
                <div>
                  <label class="mb-1.5 block text-sm font-semibold text-gray-800 dark:text-gray-200" for="test-center-delay">
                    {{ t('admin.testCenter.delay') }}
                  </label>
                  <div class="flex items-center gap-2">
                    <input id="test-center-delay" v-model.number="delaySeconds" type="number" min="0" max="30" class="input w-full" :disabled="isRunning" />
                    <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.testCenter.seconds') }}</span>
                  </div>
                  <p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.testCenter.serialHint') }}</p>
                </div>
              </div>

              <div class="rounded-xl bg-gray-50 px-3 py-2.5 text-xs text-gray-600 dark:bg-dark-700/70 dark:text-gray-300">
                {{ t('admin.testCenter.estimated', { count: estimatedRuns }) }}
              </div>

              <div class="flex gap-2">
                <button type="button" class="btn btn-primary flex-1" :disabled="runDisabled" @click="startRun">
                  <Icon v-if="!isRunning" name="play" size="sm" />
                  <Icon v-else name="refresh" size="sm" class="animate-spin" />
                  {{ isRunning ? t('admin.testCenter.running') : t('admin.testCenter.run') }}
                </button>
                <button v-if="isRunning" type="button" class="btn btn-secondary" @click="stopRun">
                  {{ t('admin.testCenter.stop') }}
                </button>
              </div>
              <div v-if="isRunning" class="space-y-1.5">
                <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400">
                  <span class="truncate">{{ currentRunLabel }}</span>
                  <span class="shrink-0">{{ results.length }}/{{ estimatedRuns }}</span>
                </div>
                <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600">
                  <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progressPercent}%` }" />
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="space-y-6">
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div v-for="stat in summaryStats" :key="stat.label" class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</div>
              <div class="mt-1 text-xl font-black" :class="stat.className">{{ stat.value }}</div>
            </div>
          </div>

          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h2 class="text-base font-bold text-gray-900 dark:text-white">{{ t('admin.testCenter.results') }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.testCenter.resultHint') }}</p>
              </div>
              <button v-if="results.length && !isRunning" type="button" class="text-xs text-gray-500 hover:text-red-500 dark:text-gray-400" @click="results = []">
                {{ t('admin.testCenter.clearResults') }}
              </button>
            </div>

            <div v-if="results.length === 0" class="mt-5 rounded-2xl border border-dashed border-gray-200 px-4 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              {{ t('admin.testCenter.noResults') }}
            </div>
            <div v-else class="mt-4 space-y-3">
              <article v-for="(result, index) in reversedResults" :key="`${result.accountId}-${index}-${result.durationMs}`" class="rounded-2xl border border-gray-200 p-4 dark:border-dark-600">
                <div class="flex flex-wrap items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="truncate text-sm font-bold text-gray-900 dark:text-white">{{ result.accountName }}</span>
                      <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] uppercase text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ result.platform }}</span>
                      <span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" :class="statusClass(result.status)">{{ statusLabel(result.status) }}</span>
                    </div>
                    <div class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">{{ result.modelId }}</div>
                  </div>
                  <span class="shrink-0 text-xs font-medium text-gray-500 dark:text-gray-400">{{ formatDuration(result.durationMs) }}</span>
                </div>

                <p v-if="result.error" class="mt-3 rounded-xl bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ result.error }}</p>
                <pre v-if="result.output" class="mt-3 max-h-32 overflow-auto whitespace-pre-wrap rounded-xl bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-700 dark:bg-dark-700/70 dark:text-gray-200">{{ result.output }}</pre>
                <div v-if="result.imageUrls.length" class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
                  <a v-for="(imageUrl, imageIndex) in result.imageUrls" :key="`${imageUrl}-${imageIndex}`" :href="imageUrl" target="_blank" rel="noreferrer" class="group overflow-hidden rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700">
                    <img :src="imageUrl" :alt="t('admin.testCenter.imagePreview')" class="aspect-square w-full object-cover transition-transform group-hover:scale-105" />
                  </a>
                </div>
                <div v-if="!result.output && !result.imageUrls.length && !result.error" class="mt-3 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.testCenter.noOutput') }}
                </div>
              </article>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { AccountListItem, ClaudeModel } from '@/types'
import { isImageModel, isVideoModel, runAccountTest, type AccountTestRunResult } from './testCenterRunner'

const { t } = useI18n()

type TestTab = 'qa' | 'image'

const testTab = ref<TestTab>('qa')
const accounts = ref<AccountListItem[]>([])
const selectedAccountIds = ref<number[]>([])
const selectedModelId = ref('')
const prompt = ref('请只回复：稳定性测试通过')
const rounds = ref(3)
const delaySeconds = ref(0)
const results = ref<AccountTestRunResult[]>([])
const loadingAccounts = ref(false)
const loadingModels = ref(false)
const loadError = ref('')
const currentRunLabel = ref('')
const isRunning = ref(false)
const runAbortController = ref<AbortController | null>(null)
const availableModels = ref<ClaudeModel[]>([])
const modelCache = new Map<number, ClaudeModel[]>()
let modelLoadSequence = 0

const primaryAccount = computed(() => accounts.value.find((account) => account.id === selectedAccountIds.value[0]) || null)
const maxRounds = computed(() => (testTab.value === 'qa' ? 10 : 3))
const modelOptions = computed(() => {
  const platform = primaryAccount.value?.platform || ''
  return availableModels.value.filter((model) => {
    if (testTab.value === 'image') return isImageModel(model.id, platform)
    return !isImageModel(model.id, platform) && !isVideoModel(model.id, platform)
  })
})
const estimatedRuns = computed(() => selectedAccountIds.value.length * Math.min(Math.max(Number(rounds.value) || 1, 1), maxRounds.value))
const completedResults = computed(() => results.value.filter((result) => result.status !== 'aborted'))
const successfulResults = computed(() => completedResults.value.filter((result) => result.status === 'success'))
const failedResults = computed(() => completedResults.value.filter((result) => result.status === 'error'))
const successRate = computed(() => completedResults.value.length ? Math.round((successfulResults.value.length / completedResults.value.length) * 100) : 0)
const averageLatency = computed(() => {
  if (!completedResults.value.length) return 0
  return Math.round(completedResults.value.reduce((sum, result) => sum + result.durationMs, 0) / completedResults.value.length)
})
const p95Latency = computed(() => {
  if (!completedResults.value.length) return 0
  const sorted = completedResults.value.map((result) => result.durationMs).sort((a, b) => a - b)
  return sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * 0.95) - 1)] || 0
})
const progressPercent = computed(() => estimatedRuns.value ? Math.min(100, Math.round((results.value.length / estimatedRuns.value) * 100)) : 0)
const reversedResults = computed(() => [...results.value].reverse())
const runDisabled = computed(() => isRunning.value || selectedAccountIds.value.length === 0 || !selectedModelId.value.trim() || !prompt.value.trim() || estimatedRuns.value === 0)
const summaryStats = computed(() => [
  { label: t('admin.testCenter.total'), value: completedResults.value.length, className: 'text-gray-900 dark:text-white' },
  { label: t('admin.testCenter.success'), value: successfulResults.value.length, className: 'text-emerald-600 dark:text-emerald-400' },
  { label: t('admin.testCenter.failed'), value: failedResults.value.length, className: 'text-red-600 dark:text-red-400' },
  { label: t('admin.testCenter.successRate'), value: `${successRate.value}%`, className: successRate.value >= 90 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400' },
  { label: t('admin.testCenter.averageLatency'), value: formatDuration(averageLatency.value), className: 'text-blue-600 dark:text-blue-400' },
  { label: t('admin.testCenter.p95Latency'), value: formatDuration(p95Latency.value), className: 'text-blue-600 dark:text-blue-400' },
])

function formatDuration(milliseconds: number) {
  if (!milliseconds) return '—'
  if (milliseconds < 1000) return `${milliseconds} ms`
  return `${(milliseconds / 1000).toFixed(1)} s`
}

function statusLabel(status: AccountTestRunResult['status']) {
  return status === 'success'
    ? t('admin.testCenter.successStatus')
    : status === 'aborted'
      ? t('admin.testCenter.abortedStatus')
      : t('admin.testCenter.failedStatus')
}

function statusClass(status: AccountTestRunResult['status']) {
  return status === 'success'
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    : status === 'aborted'
      ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
      : 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
}

function defaultModelFor(platform: string, tab: TestTab) {
  if (tab === 'image') {
    if (platform === 'openai') return 'gpt-image-1'
    if (platform === 'gemini' || platform === 'antigravity') return 'gemini-2.5-flash-image'
    if (platform === 'grok') return 'grok-imagine-image'
  }
  if (platform === 'openai') return 'gpt-5.4'
  if (platform === 'grok') return 'grok-4-1-fast'
  return ''
}

async function loadAccounts() {
  loadingAccounts.value = true
  loadError.value = ''
  try {
    const firstPage = await adminAPI.accounts.list(1, 100, { status: 'active', lite: '1' })
    const loaded = [...(firstPage.items || [])]
    const pageCount = Math.min(firstPage.pages || 1, 10)
    for (let page = 2; page <= pageCount; page += 1) {
      const nextPage = await adminAPI.accounts.list(page, 100, { status: 'active', lite: '1' })
      loaded.push(...(nextPage.items || []))
    }
    accounts.value = loaded
    const availableIds = new Set(loaded.map((account) => account.id))
    selectedAccountIds.value = selectedAccountIds.value.filter((id) => availableIds.has(id))
    if (!selectedAccountIds.value.length && loaded[0]) selectedAccountIds.value = [loaded[0].id]
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : t('admin.testCenter.loadFailed')
  } finally {
    loadingAccounts.value = false
  }
}

function selectAllAccounts() {
  selectedAccountIds.value = accounts.value.map((account) => account.id)
}

function clearAccounts() {
  selectedAccountIds.value = []
  availableModels.value = []
  selectedModelId.value = ''
}

function toggleAccount(accountId: number) {
  if (selectedAccountIds.value.includes(accountId)) {
    selectedAccountIds.value = selectedAccountIds.value.filter((id) => id !== accountId)
  } else {
    selectedAccountIds.value = [...selectedAccountIds.value, accountId]
  }
}

async function refreshModels() {
  const accountId = selectedAccountIds.value[0]
  const account = primaryAccount.value
  const sequence = ++modelLoadSequence
  if (!accountId || !account) {
    availableModels.value = []
    selectedModelId.value = ''
    return
  }
  loadingModels.value = true
  try {
    const cached = modelCache.get(accountId)
    const models = cached || await adminAPI.accounts.getAvailableModels(accountId)
    if (!cached) modelCache.set(accountId, models)
    if (sequence !== modelLoadSequence) return
    availableModels.value = models
    const compatible = models.filter((model) => testTab.value === 'image'
      ? isImageModel(model.id, account.platform)
      : !isImageModel(model.id, account.platform) && !isVideoModel(model.id, account.platform))
    if (!compatible.some((model) => model.id === selectedModelId.value)) {
      selectedModelId.value = compatible[0]?.id || defaultModelFor(account.platform, testTab.value)
    }
  } catch (error) {
    if (sequence === modelLoadSequence) {
      availableModels.value = []
      selectedModelId.value = defaultModelFor(account.platform, testTab.value)
      console.warn('Failed to load upstream models:', error)
    }
  } finally {
    if (sequence === modelLoadSequence) loadingModels.value = false
  }
}

function requestMode(account: AccountListItem): string | undefined {
  if (account.platform === 'grok') return testTab.value === 'image' ? 'image' : 'text'
  if (account.platform === 'openai') return 'default'
  return undefined
}

async function startRun() {
  if (runDisabled.value) return
  const selectedAccounts = accounts.value.filter((account) => selectedAccountIds.value.includes(account.id))
  const totalRounds = Math.min(Math.max(Number(rounds.value) || 1, 1), maxRounds.value)
  rounds.value = totalRounds
  results.value = []
  isRunning.value = true
  const controller = new AbortController()
  runAbortController.value = controller

  try {
    for (const account of selectedAccounts) {
      for (let round = 1; round <= totalRounds; round += 1) {
        if (controller.signal.aborted) return
        currentRunLabel.value = `${account.name} · ${t('admin.testCenter.request')} ${round}/${totalRounds}`
        const result = await runAccountTest({
          account,
          modelId: selectedModelId.value.trim(),
          prompt: prompt.value.trim(),
          mode: requestMode(account),
          signal: controller.signal,
        })
        results.value.push(result)
        if (result.status === 'aborted') return
        if (delaySeconds.value > 0 && (round < totalRounds || account.id !== selectedAccounts[selectedAccounts.length - 1]?.id)) {
          await new Promise((resolve) => setTimeout(resolve, Math.min(Number(delaySeconds.value) || 0, 30) * 1000))
        }
      }
    }
  } finally {
    isRunning.value = false
    runAbortController.value = null
    currentRunLabel.value = ''
  }
}

function stopRun() {
  runAbortController.value?.abort()
}

watch([selectedAccountIds, testTab], () => {
  if (!isRunning.value) void refreshModels()
}, { deep: true })

watch(testTab, () => {
  rounds.value = Math.min(rounds.value, maxRounds.value)
  prompt.value = testTab.value === 'image' ? '一只橙色猫宇航员贴纸，纯色背景' : '请只回复：稳定性测试通过'
})

onMounted(() => {
  void loadAccounts()
})

onUnmounted(() => {
  runAbortController.value?.abort()
})
</script>
