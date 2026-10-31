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
          <button type="button" class="btn border-white/30 bg-white/15 text-white hover:bg-white/25" :disabled="loading" @click="load">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-2">{{ t('common.refresh') }}</span>
          </button>
        </div>
      </header>

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
            <Select v-model="form.interval_seconds" :options="intervalOptions" />
          </div>
          <div class="flex items-center justify-between rounded-xl border border-gray-200 px-4 dark:border-dark-700">
            <div>
              <div class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.channelIQ.config.enabledLabel') }}</div>
              <div class="mt-0.5 text-xs text-gray-400">{{ t('admin.channelIQ.config.enabledHint') }}</div>
            </div>
            <Toggle v-model="form.enabled" />
          </div>
          <div class="lg:col-span-2">
            <label class="input-label">{{ t('admin.channelIQ.config.prompt') }}</label>
            <textarea v-model="form.prompt" rows="5" class="input resize-y" :placeholder="t('admin.channelIQ.config.promptPlaceholder')"></textarea>
            <p class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">{{ t('admin.channelIQ.config.promptHint') }}</p>
          </div>
        </div>

        <div class="mt-5 flex flex-wrap justify-end gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
          <button type="button" class="btn btn-secondary" :disabled="saving || running" @click="runNow">
            <Icon name="refresh" size="sm" :class="running ? 'animate-spin' : ''" />
            <span class="ml-2">{{ t('admin.channelIQ.config.runNow') }}</span>
          </button>
          <button type="button" class="btn btn-primary" :disabled="saving || !form.account_id" @click="save">
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

        <div v-if="results.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <article v-for="result in results" :key="result.id" class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="absolute inset-x-0 top-0 h-1" :class="result.status === 'success' ? 'bg-emerald-500' : 'bg-rose-500'"></div>
            <div class="flex items-start justify-between gap-3">
              <div>
                <div class="text-xs text-gray-400">{{ formatTime(result.created_at) }}</div>
                <div class="mt-2 flex flex-wrap gap-1.5 text-xs">
                  <span class="rounded-full bg-violet-50 px-2 py-1 font-medium text-violet-700 dark:bg-violet-900/25 dark:text-violet-300">{{ result.model_id || t('admin.channelIQ.results.autoModel') }}</span>
                  <span class="rounded-full bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ result.reasoning_effort || t('admin.channelIQ.results.autoReasoning') }}</span>
                </div>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="result.status === 'success' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300' : 'bg-rose-50 text-rose-700 dark:bg-rose-900/25 dark:text-rose-300'">
                {{ result.status === 'success' ? t('admin.channelIQ.results.success') : t('admin.channelIQ.results.failed') }}
              </span>
            </div>
            <iframe
              v-if="result.status === 'success' && hasAnimation(result.response_text)"
              :srcdoc="renderAnimation(result.response_text)"
              class="mt-4 h-56 w-full rounded-xl border-0 bg-gray-50 dark:bg-dark-900/70"
              sandbox=""
              :title="t('admin.channelIQ.results.animationTitle')"
            ></iframe>
            <p v-else-if="result.status === 'success'" class="mt-4 rounded-xl bg-amber-50 px-3 py-2.5 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.channelIQ.results.animationUnavailable') }}</p>
            <p v-else class="mt-4 max-h-24 overflow-auto rounded-xl bg-rose-50 px-3 py-2.5 text-sm text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">{{ result.error_message || t('admin.channelIQ.results.empty') }}</p>
            <div class="mt-4 flex items-center justify-between text-xs text-gray-400">
              <span>{{ result.latency_ms }} ms</span>
              <span>{{ formatDate(result.finished_at) }}</span>
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
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { AccountListItem } from '@/types'
import type { ChannelIQResult } from '@/api/admin/channelIQ'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { extractChannelIQAnimationDocument } from '@/utils/channelIQHtml'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const running = ref(false)
const accountsLoading = ref(false)
const accounts = ref<AccountListItem[]>([])
const results = ref<ChannelIQResult[]>([])
const form = reactive({ account_id: 0, model_id: '', prompt: '', reasoning_effort: '', interval_seconds: 300, enabled: false })

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
  { value: 60, label: t('admin.channelIQ.config.intervalOptions.minute') },
  { value: 300, label: t('admin.channelIQ.config.intervalOptions.fiveMinutes') },
  { value: 900, label: t('admin.channelIQ.config.intervalOptions.fifteenMinutes') },
  { value: 1800, label: t('admin.channelIQ.config.intervalOptions.thirtyMinutes') },
  { value: 3600, label: t('admin.channelIQ.config.intervalOptions.hour') },
  { value: 86400, label: t('admin.channelIQ.config.intervalOptions.day') },
])

function applySnapshot(snapshot: Awaited<ReturnType<typeof adminAPI.channelIQ.get>>) {
  results.value = snapshot.results || []
  if (snapshot.config) {
    form.account_id = snapshot.config.account_id
    form.model_id = snapshot.config.model_id || ''
    form.prompt = snapshot.config.prompt || ''
    form.reasoning_effort = snapshot.config.reasoning_effort || ''
    form.interval_seconds = snapshot.config.interval_seconds || 300
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

async function load() {
  loading.value = true
  try {
    const snapshot = await adminAPI.channelIQ.get()
    applySnapshot(snapshot)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.loadError')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!form.account_id) return
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

async function runNow() {
  if (running.value) return
  running.value = true
  try {
    const result = await adminAPI.channelIQ.run()
    results.value = [result, ...results.value.filter(item => item.id !== result.id)].slice(0, 6)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelIQ.runError')))
  } finally {
    running.value = false
  }
}

function renderAnimation(value: string): string {
  return extractChannelIQAnimationDocument(value)
}

function hasAnimation(value: string): boolean {
  return renderAnimation(value) !== ''
}

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '—'
}

function formatDate(value?: string): string {
  return value ? new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '—'
}

onMounted(async () => {
  await Promise.all([loadAccounts(), load()])
})
</script>
