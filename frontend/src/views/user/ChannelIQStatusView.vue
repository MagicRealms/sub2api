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

      <section v-if="loading && results.length === 0" class="grid gap-4 md:grid-cols-2 xl:grid-cols-2">
        <div v-for="n in 6" :key="n" class="h-64 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
      </section>

      <section v-else-if="!configured || results.length === 0" class="card p-10 text-center">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-violet-50 text-violet-500 dark:bg-violet-900/20 dark:text-violet-300">
          <Icon name="sparkles" size="lg" />
        </div>
        <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('channelIQ.empty.title') }}</h2>
        <p class="mx-auto mt-2 max-w-lg text-sm text-gray-500 dark:text-gray-400">{{ t('channelIQ.empty.description') }}</p>
      </section>

      <section v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-2">
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
              {{ result.status === 'success' ? t('channelIQ.card.requestSuccess') : t('channelIQ.status.error') }}
            </span>
          </div>

          <div
            v-if="result.status === 'success' && hasAnimation(result.response_text)"
            :style="animationStyle(result.response_text)"
            class="relative mt-4 w-full overflow-hidden rounded-xl bg-transparent"
          >
            <iframe
              :srcdoc="renderAnimation(result.response_text)"
              class="absolute inset-0 h-full w-full border-0 bg-transparent"
              sandbox=""
              :title="t('channelIQ.card.animationTitle')"
            ></iframe>
          </div>
          <p v-else-if="result.status === 'success'" class="mt-4 rounded-xl bg-amber-50 px-3 py-2.5 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">{{ t('channelIQ.card.animationUnavailable') }}</p>
          <p v-else class="mt-4 max-h-24 overflow-auto rounded-xl bg-rose-50 px-3 py-2.5 text-sm text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">{{ result.error_message || t('channelIQ.card.waiting') }}</p>
          <div v-if="result.evaluation" class="mt-4 rounded-xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-900/50">
            <div class="flex items-center justify-between gap-3 text-xs">
              <span class="font-semibold text-gray-700 dark:text-gray-200">{{ t('channelIQ.card.rubricTitle') }}</span>
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
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import channelIQAPI, { type ChannelIQCriterionStatus, type ChannelIQResult } from '@/api/channelIQ'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { extractChannelIQAnimationDocument, getChannelIQAnimationAspectRatio } from '@/utils/channelIQHtml'

const { t } = useI18n()
const appStore = useAppStore()
const results = ref<ChannelIQResult[]>([])
const configured = ref(false)
const loading = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | undefined

function renderAnimation(value: string): string {
  return extractChannelIQAnimationDocument(value)
}

function hasAnimation(value: string): boolean {
  return renderAnimation(value) !== ''
}

function animationStyle(value: string): { aspectRatio: string } {
  return { aspectRatio: String(getChannelIQAnimationAspectRatio(value)) }
}

function evaluationGradeLabel(result: ChannelIQResult): string {
  const evaluation = result.evaluation
  if (!evaluation?.graded) return t('channelIQ.card.grades.ungraded')
  return t(`channelIQ.card.grades.${evaluation.grade}`)
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
    { key: 'wheels', label: t('channelIQ.card.criteria.wheels'), status: criteria.wheels.status },
    { key: 'leg_pedal', label: t('channelIQ.card.criteria.legPedal'), status: criteria.leg_pedal.status },
    { key: 'view_consistent', label: t('channelIQ.card.criteria.viewConsistent'), status: criteria.view_consistent.status },
    { key: 'pelican_beak', label: t('channelIQ.card.criteria.pelicanBeak'), status: criteria.pelican_beak.status },
  ]
}

function criterionStatusLabel(status: ChannelIQCriterionStatus): string {
  return t(`channelIQ.card.criterionStatus.${status}`)
}

function criterionStatusClass(status: ChannelIQCriterionStatus): string {
  if (status === 'pass') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'partial') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (status === 'fail') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  return 'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
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
