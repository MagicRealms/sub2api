<template>
  <BaseDialog :show="show" :title="t('admin.users.usageDetails.title')" width="wide" @close="emit('close')">
    <div class="console-usage-dialog">
      <div v-if="displayUser" class="sw-user-identity">
        <div class="sw-user-avatar">{{ (displayUser.username || displayUser.email || '?').slice(0,1).toUpperCase() }}</div>
        <div class="sw-user-identity-main"><strong :title="displayUser.email">{{ displayUser.email }}</strong><small>#{{ displayUser.id }} · {{ displayUser.username || '—' }} · {{ t(`admin.users.usageDetails.${displayUser.status === 'active' ? 'active' : 'disabled'}`) }}</small></div>
        <div class="sw-user-balance"><span>{{ t('admin.users.usageDetails.balance') }}</span><strong>{{ money(displayUser.balance) }}</strong></div>
      </div>
      <div v-if="loading" role="status" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
        <p>{{ t('admin.users.usageDetails.loadFailed') }}</p><button class="btn btn-secondary mt-3" @click="loadDetails">{{ t('admin.users.usageDetails.retry') }}</button>
      </div>
      <template v-else-if="details">
        <section>
          <div class="sw-section-head"><h3><WorkspaceIcon name="analysis" />{{ t('admin.users.usageDetails.sevenDays') }}</h3><button class="sw-section-link" @click="loadDetails">{{ t('admin.users.usageDetails.refresh') }}</button></div>
          <div class="sw-day-strip" role="group" :aria-label="t('admin.users.usageDetails.selectDay')">
            <button v-for="day in daysNewestFirst" :key="day.date" :data-test="`usage-day-${day.date}`" :title="`${day.date} · ${number(day.requests)}`" :aria-label="day.date" :aria-pressed="selectedDate === day.date" class="sw-day" :class="{ 'is-selected': selectedDate === day.date }" @click="selectedDate = day.date; page = 1">
              <span class="sw-day-date">{{ day.date === details.end_date ? t('admin.users.usageDetails.today') : day.date.slice(5) }}</span>
              <span class="sw-day-meter" aria-hidden="true"><i :style="{ height: `${Math.max(8, day.requests / maxDayRequests * 100)}%` }"></i></span>
              <span class="sw-day-count">{{ number(day.requests) }}</span>
            </button>
          </div>
          <p class="sw-section-meta mt-2">{{ details.timezone }} · {{ selectedDate }}</p>
        </section>
        <section class="sw-usage-summary" data-test="usage-summary" :aria-label="t('admin.users.usageDetails.todayUsage')">
          <MetricCard :label="t('admin.users.usageDetails.requests')" icon="pulse" tone="sky">
            <template #value><span data-test="day-requests">{{ number(selectedUsage.requests) }}</span></template>
            <div class="sw-total-comparison"><span>{{ t('admin.users.usageDetails.totalUsage') }}</span><b data-test="total-requests">{{ number(details.total.requests) }}</b></div>
          </MetricCard>
          <MetricCard label="Token" icon="models" tone="violet">
            <template #value><span data-test="day-tokens" :title="tokenBreakdown(selectedUsage)">{{ formatTokensM(selectedUsage.total_tokens) }}</span></template>
            <div class="sw-total-comparison" :title="tokenBreakdown(details.total)"><span>{{ t('admin.users.usageDetails.totalUsage') }}</span><b data-test="total-tokens">{{ formatTokensM(details.total.total_tokens) }}</b></div>
          </MetricCard>
          <MetricCard :label="t('admin.users.usageDetails.actualCost')" icon="wallet" tone="mint">
            <template #value><span data-test="day-cost">{{ money(selectedUsage.actual_cost) }}</span></template>
            <div class="sw-total-comparison"><span>{{ t('admin.users.usageDetails.totalUsage') }}</span><b data-test="total-cost">{{ money(details.total.actual_cost) }}</b></div>
          </MetricCard>
        </section>
        <section>
          <div class="sw-section-head"><h3><WorkspaceIcon name="key" />{{ t('admin.users.usageDetails.apiKeys') }} <small class="text-gray-400">{{ details.api_keys.length }}</small></h3><input v-model="search" type="search" class="input w-36 sm:w-48" :placeholder="t('admin.users.usageDetails.searchKeys')" :aria-label="t('admin.users.usageDetails.searchKeys')" @input="page = 1" /></div>
          <div class="sw-key-usage-list" data-test="usage-keys">
            <article v-for="key in pagedKeys" :key="key.id" :data-test="`usage-key-${key.id}`" class="sw-key-usage-item">
              <div class="sw-key-identity"><strong :title="`${key.name} · ${key.group_name || ''}`">{{ key.name || `#${key.id}` }}</strong><small class="truncate" :title="`${key.key_preview || ''} · ${key.group_name || ''}`">{{ key.key_preview || '—' }}</small><small :title="`${t('admin.users.usageDetails.lastUsed')} ${dateTime(key.last_used_at)}`">{{ t(`admin.users.usageDetails.${keyStatus(key)}`) }}<span v-if="key.group_name"> · {{ key.group_name }}</span></small></div>
              <div class="sw-key-metric" :title="tokenBreakdown(keyDay(key))"><span>{{ t('admin.users.usageDetails.dayUsage') }}</span><b>{{ money(keyDay(key).actual_cost) }}</b><em>{{ number(keyDay(key).requests) }} / {{ formatTokensM(keyDay(key).total_tokens) }}</em></div>
              <div class="sw-key-metric" :title="tokenBreakdown(key.total)"><span>{{ t('admin.users.usageDetails.totalUsage') }}</span><b>{{ money(key.total.actual_cost) }}</b><em>{{ number(key.total.requests) }} / {{ formatTokensM(key.total.total_tokens) }}</em></div>
            </article>
            <div v-if="filteredKeys.length === 0" class="py-8 text-center text-xs text-gray-400">{{ t('admin.users.usageDetails.noKeys') }}</div>
          </div>
          <div v-if="filteredKeys.length > pageSize" class="mt-3 flex items-center justify-end gap-3 text-xs"><button class="btn btn-secondary" :disabled="page === 1" :aria-label="t('admin.users.usageDetails.previous')" @click="page--">‹</button><span>{{ page }} / {{ pageCount }}</span><button class="btn btn-secondary" :disabled="page >= pageCount" :aria-label="t('admin.users.usageDetails.next')" @click="page++">›</button></div>
        </section>
        <details v-if="displayUser" class="sw-account-extra">
          <summary>{{ t('admin.users.usageDetails.limits') }} · {{ t('admin.users.usageDetails.createdAt') }} · {{ t('admin.users.usageDetails.notes') }}</summary>
          <dl>
            <div><dt>{{ t('admin.users.usageDetails.limits') }}</dt><dd>{{ t('admin.users.usageDetails.limitValues', { concurrency: displayUser.concurrency, rpm: displayUser.rpm_limit || t('admin.users.usageDetails.unlimited') }) }}</dd></div>
            <div><dt>{{ t('admin.users.usageDetails.createdAt') }}</dt><dd>{{ dateTime(displayUser.created_at) }}</dd></div>
            <div><dt>{{ t('admin.users.usageDetails.lastActive') }}</dt><dd>{{ dateTime(displayUser.last_active_at) }}</dd></div>
            <div><dt>{{ t('admin.users.usageDetails.lastUsed') }}</dt><dd>{{ dateTime(displayUser.last_used_at) }}</dd></div>
            <div v-if="displayUser.notes" class="col-span-full"><dt>{{ t('admin.users.usageDetails.notes') }}</dt><dd class="break-words">{{ displayUser.notes }}</dd></div>
          </dl>
        </details>
        <p class="sw-section-meta">{{ t('admin.users.usageDetails.totalHint') }} · {{ t('admin.users.usageDetails.updatedAt') }} {{ dateTime(details.generated_at) }}</p>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { UserAPIKeyUsage, UserUsageDetails, UserUsageMetrics } from '@/api/admin/users'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import MetricCard from '@/components/workspace/MetricCard.vue'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
import { formatTokenMillions as formatTokensM } from '@/utils/workspace'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t, locale } = useI18n()
const details = ref<UserUsageDetails | null>(null)
const loading = ref(false)
const error = ref(false)
const selectedDate = ref('')
const search = ref('')
const page = ref(1)
const pageSize = 8
let controller: AbortController | null = null
let requestVersion = 0
const zero: UserUsageMetrics = { requests: 0, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, actual_cost: 0 }
const displayUser = computed(() => details.value?.user ?? props.user)
const daysNewestFirst = computed(() => [...(details.value?.daily ?? [])].reverse())
const maxDayRequests = computed(() => Math.max(1, ...(details.value?.daily ?? []).map(d => d.requests)))
const selectedUsage = computed(() => details.value?.daily.find(day => day.date === selectedDate.value) ?? zero)
const filteredKeys = computed(() => {
  const term = search.value.trim().toLocaleLowerCase()
  return (details.value?.api_keys ?? [])
    .filter(key => !term || `${key.name} ${key.key_preview} ${key.group_name} ${key.id}`.toLocaleLowerCase().includes(term))
    .slice().sort((a, b) => keyDay(b).actual_cost - keyDay(a).actual_cost || keyDay(b).requests - keyDay(a).requests || b.total.actual_cost - a.total.actual_cost || a.id - b.id)
})
const pageCount = computed(() => Math.max(1, Math.ceil(filteredKeys.value.length / pageSize)))
const pagedKeys = computed(() => filteredKeys.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const number = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const money = (value: number) => new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value)
const dateTime = (value?: string | null) => value ? formatDateTime(value) : '—'
const keyDay = (key: UserAPIKeyUsage): UserUsageMetrics => key.daily.find(day => day.date === selectedDate.value) ?? zero
const keyStatus = (key: UserAPIKeyUsage) => {
  if (key.deleted) return 'deleted'
  if (key.expires_at && Date.parse(key.expires_at) <= Date.parse(details.value?.generated_at ?? '')) return 'expired'
  if (key.status === 'quota_exhausted') return 'quotaExhausted'
  return key.status === 'active' ? 'active' : key.status === 'expired' ? 'expired' : 'disabled'
}
const tokenBreakdown = (usage: UserUsageMetrics) => t('admin.users.usageDetails.tokenBreakdown', {
  input: number(usage.input_tokens), output: number(usage.output_tokens),
  creation: number(usage.cache_creation_tokens), read: number(usage.cache_read_tokens)
})

async function loadDetails() {
  controller?.abort()
  const version = ++requestVersion
  if (!props.show || !props.user) {
    details.value = null
    loading.value = false
    return
  }
  const userID = props.user.id
  controller = new AbortController()
  loading.value = true
  error.value = false
  try {
    const result = await adminAPI.users.getUserUsageDetails(userID, {
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      signal: controller.signal
    })
    if (version !== requestVersion) return
    details.value = result
    if (!result.daily.some(day => day.date === selectedDate.value)) selectedDate.value = result.end_date
    page.value = Math.min(page.value, pageCount.value)
  } catch {
    if (version !== requestVersion) return
    details.value = null
    error.value = true
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

watch(() => [props.show, props.user?.id] as const, () => {
  details.value = null
  selectedDate.value = ''
  search.value = ''
  page.value = 1
  void loadDetails()
}, { immediate: true })
onBeforeUnmount(() => { ++requestVersion; controller?.abort() })
</script>
