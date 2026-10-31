<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <section class="relative overflow-hidden rounded-3xl bg-gradient-to-br from-indigo-600 via-violet-600 to-fuchsia-600 p-6 text-white shadow-lg sm:p-8">
        <div class="absolute -right-12 -top-16 h-48 w-48 rounded-full bg-white/10 blur-2xl" aria-hidden="true"></div>
        <div class="relative flex flex-wrap items-start justify-between gap-5">
          <div class="max-w-2xl">
            <div class="mb-3 inline-flex h-11 w-11 items-center justify-center rounded-2xl bg-white/15 ring-1 ring-white/20">
              <Icon name="sparkles" size="lg" />
            </div>
            <h1 class="text-2xl font-black tracking-tight sm:text-3xl">{{ t('channelIQ.title') }}</h1>
            <p class="mt-2 text-sm leading-6 text-white/80">{{ t('channelIQ.description') }}</p>
          </div>
          <div class="rounded-2xl bg-black/10 px-4 py-3 text-right ring-1 ring-white/10">
            <div class="text-xs text-white/65">{{ t('channelIQ.summary.records') }}</div>
            <div class="mt-1 text-2xl font-bold tabular-nums">{{ results.length }}/6</div>
          </div>
        </div>
      </section>

      <section v-if="loading && results.length === 0" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="n in 6" :key="n" class="h-64 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
      </section>

      <section v-else-if="!configured || results.length === 0" class="card p-10 text-center">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-violet-50 text-violet-500 dark:bg-violet-900/20 dark:text-violet-300">
          <Icon name="sparkles" size="lg" />
        </div>
        <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('channelIQ.empty.title') }}</h2>
        <p class="mx-auto mt-2 max-w-lg text-sm text-gray-500 dark:text-gray-400">{{ t('channelIQ.empty.description') }}</p>
      </section>

      <section v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="result in results"
          :key="result.id"
          class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="absolute inset-x-0 top-0 h-1" :class="result.status === 'success' ? 'bg-emerald-500' : 'bg-rose-500'"></div>
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-xs text-gray-400 dark:text-gray-500">{{ formatTime(result.created_at) }}</div>
              <div class="mt-2 flex flex-wrap gap-1.5 text-xs">
                <span class="rounded-full bg-violet-50 px-2 py-1 font-medium text-violet-700 dark:bg-violet-900/25 dark:text-violet-300">{{ result.model_id || t('channelIQ.card.autoModel') }}</span>
                <span class="rounded-full bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ result.reasoning_effort || t('channelIQ.card.autoReasoning') }}</span>
              </div>
            </div>
            <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="result.status === 'success' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300' : 'bg-rose-50 text-rose-700 dark:bg-rose-900/25 dark:text-rose-300'">
              {{ result.status === 'success' ? t('channelIQ.status.healthy') : t('channelIQ.status.error') }}
            </span>
          </div>

          <div v-if="result.status === 'success' && result.response_text" class="markdown-body mt-4 max-h-56 overflow-auto rounded-xl bg-gray-50 px-3 py-2.5 text-sm dark:bg-dark-900/70" v-html="renderMarkdown(result.response_text)"></div>
          <p v-else class="mt-4 max-h-24 overflow-auto rounded-xl bg-rose-50 px-3 py-2.5 text-sm text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">{{ result.error_message || t('channelIQ.card.waiting') }}</p>

          <div class="mt-4 flex items-center justify-between text-xs text-gray-400 dark:text-gray-500">
            <span>{{ result.latency_ms }} ms</span>
            <span>{{ formatTime(result.finished_at) }}</span>
          </div>
        </article>
      </section>

      <p class="text-center text-xs text-gray-400 dark:text-gray-500">{{ t('channelIQ.footnote') }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import channelIQAPI, { type ChannelIQResult } from '@/api/channelIQ'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const appStore = useAppStore()
const results = ref<ChannelIQResult[]>([])
const configured = ref(false)
const loading = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | undefined

function renderMarkdown(value: string): string {
  return DOMPurify.sanitize(marked.parse(value || '') as string)
}

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '—'
}

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const snapshot = await channelIQAPI.get()
    configured.value = snapshot.configured
    results.value = (snapshot.results || []).slice(0, 6)
  } catch (error: unknown) {
    if (!silent) appStore.showError(extractApiErrorMessage(error, t('channelIQ.loadError')))
  } finally {
    if (!silent) loading.value = false
  }
}

onMounted(async () => {
  await load()
  refreshTimer = setInterval(() => load(true), 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>
