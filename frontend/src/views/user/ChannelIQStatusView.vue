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
          <button
            type="button"
            class="btn border-white/30 bg-white/15 text-white hover:bg-white/25"
            :disabled="loading"
            @click="load()"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-2">{{ t('common.refresh') }}</span>
          </button>
        </div>
        <div class="relative mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div class="rounded-2xl bg-black/10 px-4 py-3 ring-1 ring-white/10">
            <div class="text-xs text-white/65">{{ t('channelIQ.summary.total') }}</div>
            <div class="mt-1 text-2xl font-bold tabular-nums">{{ items.length }}</div>
          </div>
          <div class="rounded-2xl bg-emerald-400/15 px-4 py-3 ring-1 ring-emerald-200/15">
            <div class="text-xs text-emerald-100/75">{{ t('channelIQ.summary.healthy') }}</div>
            <div class="mt-1 text-2xl font-bold tabular-nums">{{ countByStatus('healthy') }}</div>
          </div>
          <div class="rounded-2xl bg-amber-300/15 px-4 py-3 ring-1 ring-amber-100/15">
            <div class="text-xs text-amber-100/75">{{ t('channelIQ.summary.pending') }}</div>
            <div class="mt-1 text-2xl font-bold tabular-nums">{{ countByStatus('pending') }}</div>
          </div>
          <div class="rounded-2xl bg-rose-300/15 px-4 py-3 ring-1 ring-rose-100/15">
            <div class="text-xs text-rose-100/75">{{ t('channelIQ.summary.errors') }}</div>
            <div class="mt-1 text-2xl font-bold tabular-nums">{{ countByStatus('error') }}</div>
          </div>
        </div>
      </section>

      <section v-if="loading && items.length === 0" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="n in 3" :key="n" class="h-64 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
      </section>

      <section v-else-if="items.length === 0" class="card p-10 text-center">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-violet-50 text-violet-500 dark:bg-violet-900/20 dark:text-violet-300">
          <Icon name="sparkles" size="lg" />
        </div>
        <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('channelIQ.empty.title') }}</h2>
        <p class="mx-auto mt-2 max-w-lg text-sm text-gray-500 dark:text-gray-400">{{ t('channelIQ.empty.description') }}</p>
      </section>

      <section v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="item in items"
          :key="item.channel_id"
          class="group relative overflow-hidden rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="absolute inset-x-0 top-0 h-1" :class="statusBarClass(item.status)"></div>
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="truncate text-base font-bold text-gray-900 dark:text-white">{{ item.channel_name }}</h2>
              <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                {{ item.group_name || t('channelIQ.card.noGroup') }}
                <span v-if="item.platform" class="mx-1 text-gray-300 dark:text-dark-500">·</span>
                {{ item.platform || t('channelIQ.card.autoPlatform') }}
              </p>
            </div>
            <span class="inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusBadgeClass(item.status)">
              <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(item.status)"></span>
              {{ statusLabel(item.status) }}
            </span>
          </div>

          <div class="mt-5 grid grid-cols-2 gap-3">
            <div class="rounded-xl bg-gray-50 px-3 py-2.5 dark:bg-dark-900/70">
              <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('channelIQ.card.model') }}</div>
              <div class="mt-1 truncate text-sm font-semibold text-gray-800 dark:text-gray-200" :title="item.model || ''">{{ item.model || t('channelIQ.card.autoModel') }}</div>
            </div>
            <div class="rounded-xl bg-gray-50 px-3 py-2.5 dark:bg-dark-900/70">
              <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('channelIQ.card.latency') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-800 dark:text-gray-200">{{ item.latency_ms ? `${item.latency_ms} ms` : '—' }}</div>
            </div>
          </div>

          <div class="mt-4 min-h-16 rounded-xl border border-dashed border-gray-200 px-3 py-2.5 dark:border-dark-700">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('channelIQ.card.response') }}</div>
            <p v-if="item.response_preview" class="mt-1 line-clamp-2 text-sm leading-5 text-gray-700 dark:text-gray-300">{{ item.response_preview }}</p>
            <p v-else-if="item.error" class="mt-1 line-clamp-2 text-sm leading-5 text-rose-600 dark:text-rose-300">{{ item.error }}</p>
            <p v-else class="mt-1 text-sm text-gray-400 dark:text-gray-500">{{ t('channelIQ.card.waiting') }}</p>
          </div>

          <div class="mt-4 flex items-center justify-between gap-3 text-xs text-gray-400 dark:text-gray-500">
            <span>{{ item.checked_at ? t('channelIQ.card.checkedAt', { time: formatTime(item.checked_at) }) : t('channelIQ.card.notChecked') }}</span>
            <button
              type="button"
              class="rounded-lg px-2.5 py-1.5 font-medium text-violet-600 opacity-0 transition group-hover:opacity-100 hover:bg-violet-50 disabled:cursor-not-allowed disabled:opacity-40 dark:text-violet-300 dark:hover:bg-violet-900/20"
              :disabled="runningId === item.channel_id || item.status === 'disabled' || item.status === 'unconfigured'"
              @click="run(item.channel_id)"
            >
              <Icon name="refresh" size="xs" :class="runningId === item.channel_id ? 'animate-spin' : ''" />
              <span class="ml-1">{{ t('channelIQ.card.runNow') }}</span>
            </button>
          </div>
        </article>
      </section>

      <p class="text-center text-xs text-gray-400 dark:text-gray-500">{{ t('channelIQ.footnote') }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import channelIQAPI, { type ChannelIQItem, type ChannelIQStatus } from '@/api/channelIQ'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const items = ref<ChannelIQItem[]>([])
const loading = ref(false)
const runningId = ref<number | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | undefined

const hasPending = computed(() => items.value.some(item => item.status === 'pending'))

function countByStatus(status: ChannelIQStatus): number {
  return items.value.filter(item => item.status === status).length
}

function statusLabel(status: ChannelIQStatus): string {
  return t(`channelIQ.status.${status}`)
}

function statusBarClass(status: ChannelIQStatus): string {
  return ({ healthy: 'bg-emerald-500', pending: 'bg-amber-400', warning: 'bg-amber-500', error: 'bg-rose-500', disabled: 'bg-gray-300', unconfigured: 'bg-gray-300' } as Record<ChannelIQStatus, string>)[status]
}

function statusBadgeClass(status: ChannelIQStatus): string {
  return ({ healthy: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300', pending: 'bg-amber-50 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300', warning: 'bg-amber-50 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300', error: 'bg-rose-50 text-rose-700 dark:bg-rose-900/25 dark:text-rose-300', disabled: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400', unconfigured: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400' } as Record<ChannelIQStatus, string>)[status]
}

function statusDotClass(status: ChannelIQStatus): string {
  return ({ healthy: 'bg-emerald-500', pending: 'bg-amber-500 animate-pulse', warning: 'bg-amber-500', error: 'bg-rose-500', disabled: 'bg-gray-400', unconfigured: 'bg-gray-400' } as Record<ChannelIQStatus, string>)[status]
}

function formatTime(value: string): string {
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const response = await channelIQAPI.list()
    items.value = response.items || []
  } catch (error: unknown) {
    if (!silent) appStore.showError(extractApiErrorMessage(error, t('channelIQ.loadError')))
  } finally {
    if (!silent) loading.value = false
  }
}

async function run(channelId: number) {
  if (runningId.value) return
  runningId.value = channelId
  try {
    const result = await channelIQAPI.run(channelId)
    const index = items.value.findIndex(item => item.channel_id === channelId)
    if (index >= 0) items.value.splice(index, 1, result)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('channelIQ.runError')))
  } finally {
    runningId.value = null
  }
}

onMounted(async () => {
  await load()
  refreshTimer = setInterval(() => load(true), hasPending.value ? 8000 : 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>
