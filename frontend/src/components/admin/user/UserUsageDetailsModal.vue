<template>
  <BaseDialog :show="show" :title="t('admin.users.usageDetails.title')" width="wide" @close="emit('close')">
    <div class="space-y-4">
      <div v-if="displayUser" class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800">
        <div class="flex flex-wrap items-center gap-2">
          <span class="break-all font-medium text-gray-900 dark:text-gray-100">{{ displayUser.email }}</span>
          <span class="text-xs text-gray-500">#{{ displayUser.id }} <span v-if="displayUser.username">· {{ displayUser.username }}</span></span>
          <span :class="displayUser.status === 'active' ? 'badge-success' : 'badge-danger'" class="badge">
            {{ t(`admin.users.usageDetails.${displayUser.status === 'active' ? 'active' : 'disabled'}`) }}
          </span>
        </div>
        <dl class="mt-2 grid grid-cols-2 gap-x-4 gap-y-2 text-xs sm:grid-cols-3">
          <div><dt class="text-gray-500">{{ t('admin.users.usageDetails.balance') }}</dt><dd class="mt-0.5 font-medium">{{ money(displayUser.balance) }}</dd></div>
          <div><dt class="text-gray-500">{{ t('admin.users.usageDetails.limits') }}</dt><dd class="mt-0.5">{{ t('admin.users.usageDetails.limitValues', { concurrency: displayUser.concurrency, rpm: displayUser.rpm_limit || t('admin.users.usageDetails.unlimited') }) }}</dd></div>
          <div><dt class="text-gray-500">{{ t('admin.users.usageDetails.createdAt') }}</dt><dd class="mt-0.5">{{ dateTime(displayUser.created_at) }}</dd></div>
          <div><dt class="text-gray-500">{{ t('admin.users.usageDetails.lastActive') }}</dt><dd class="mt-0.5">{{ dateTime(displayUser.last_active_at) }}</dd></div>
          <div><dt class="text-gray-500">{{ t('admin.users.usageDetails.lastUsed') }}</dt><dd class="mt-0.5">{{ dateTime(displayUser.last_used_at) }}</dd></div>
          <div v-if="displayUser.notes" class="col-span-2 sm:col-span-1"><dt class="text-gray-500">{{ t('admin.users.usageDetails.notes') }}</dt><dd class="mt-0.5 line-clamp-2 break-words" :title="displayUser.notes">{{ displayUser.notes }}</dd></div>
        </dl>
      </div>

      <div v-if="loading" role="status" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
        <p>{{ t('admin.users.usageDetails.loadFailed') }}</p>
        <button class="btn btn-secondary mt-3" @click="loadDetails">{{ t('admin.users.usageDetails.retry') }}</button>
      </div>
      <template v-else-if="details">
        <div>
          <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500">
            <span>{{ t('admin.users.usageDetails.sevenDays') }} · {{ details.timezone }}</span>
            <button class="hover:text-primary-600" @click="loadDetails">{{ t('admin.users.usageDetails.refresh') }}</button>
          </div>
          <div class="grid grid-cols-7 gap-1 sm:gap-2" role="group" :aria-label="t('admin.users.usageDetails.selectDay')">
            <button
              v-for="day in daysNewestFirst" :key="day.date" :data-test="`usage-day-${day.date}`"
              :title="day.date" :aria-label="day.date"
              :aria-pressed="selectedDate === day.date"
              :class="selectedDate === day.date ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'border-gray-200 text-gray-600 hover:border-primary-300 dark:border-dark-600 dark:text-dark-300'"
              class="rounded-lg border px-1 py-2 text-center text-xs transition-colors" @click="selectedDate = day.date; page = 1"
            >
              <span class="block font-medium">{{ day.date === details.end_date ? t('admin.users.usageDetails.today') : day.date.slice(5) }}</span>
              <span class="mt-1 block text-[11px] opacity-75">{{ number(day.requests) }}</span>
            </button>
          </div>
        </div>

        <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
          <table class="w-full text-sm" data-test="usage-summary">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-2 text-left font-medium">{{ t('admin.users.usageDetails.metric') }}</th>
                <th class="px-4 py-2 text-right font-medium">{{ selectedDate === details.end_date ? t('admin.users.usageDetails.todayUsage') : selectedDate }}</th>
                <th class="px-4 py-2 text-right font-medium" :title="t('admin.users.usageDetails.totalHint')">{{ t('admin.users.usageDetails.totalUsage') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr><th class="px-4 py-2.5 text-left font-normal text-gray-500">{{ t('admin.users.usageDetails.requests') }}</th><td class="px-4 py-2.5 text-right font-semibold tabular-nums" data-test="day-requests">{{ number(selectedUsage.requests) }}</td><td class="px-4 py-2.5 text-right tabular-nums" data-test="total-requests">{{ number(details.total.requests) }}</td></tr>
              <tr><th class="px-4 py-2.5 text-left font-normal text-gray-500">Token</th><td class="px-4 py-2.5 text-right font-semibold tabular-nums" :title="tokenBreakdown(selectedUsage)" data-test="day-tokens">{{ number(selectedUsage.total_tokens) }}</td><td class="px-4 py-2.5 text-right tabular-nums" :title="tokenBreakdown(details.total)" data-test="total-tokens">{{ number(details.total.total_tokens) }}</td></tr>
              <tr><th class="px-4 py-2.5 text-left font-normal text-gray-500">{{ t('admin.users.usageDetails.actualCost') }}</th><td class="px-4 py-2.5 text-right font-semibold tabular-nums" data-test="day-cost">{{ money(selectedUsage.actual_cost) }}</td><td class="px-4 py-2.5 text-right tabular-nums" data-test="total-cost">{{ money(details.total.actual_cost) }}</td></tr>
            </tbody>
          </table>
        </div>

        <div>
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h4 class="text-sm font-semibold">{{ t('admin.users.usageDetails.apiKeys') }} <span class="font-normal text-gray-500">({{ details.api_keys.length }})</span></h4>
            <input v-model="search" type="search" class="input w-full sm:w-48" :placeholder="t('admin.users.usageDetails.searchKeys')" :aria-label="t('admin.users.usageDetails.searchKeys')" @input="page = 1" />
          </div>
          <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
            <table class="w-full whitespace-nowrap text-xs" data-test="usage-keys">
              <thead class="bg-gray-50 text-gray-500 dark:bg-dark-800">
                <tr>
                  <th class="px-3 py-2 text-left font-medium">{{ t('admin.users.usageDetails.key') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('admin.users.usageDetails.dayUsage') }}<span class="mt-0.5 block font-normal">{{ t('admin.users.usageDetails.keyMetrics') }}</span></th>
                  <th class="px-3 py-2 text-right font-medium" :title="t('admin.users.usageDetails.totalHint')">{{ t('admin.users.usageDetails.totalUsage') }}<span class="mt-0.5 block font-normal">{{ t('admin.users.usageDetails.keyMetrics') }}</span></th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('admin.users.usageDetails.lastUsed') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="key in pagedKeys" :key="key.id" :data-test="`usage-key-${key.id}`">
                  <td class="max-w-52 px-3 py-3">
                    <div class="flex items-center gap-2"><span class="truncate font-medium" :title="key.name">{{ key.name || `#${key.id}` }}</span><span :class="keyStatus(key) === 'active' ? 'text-green-600 dark:text-green-400' : 'text-gray-500'">{{ t(`admin.users.usageDetails.${keyStatus(key)}`) }}</span></div>
                    <div class="mt-1 truncate text-gray-500"><span class="font-mono">{{ key.key_preview || '—' }}</span><span v-if="key.group_name"> · {{ key.group_name }}</span></div>
                  </td>
                  <td class="px-3 py-3 text-right tabular-nums" :title="tokenBreakdown(keyDay(key))"><div>{{ number(keyDay(key).requests) }} / {{ number(keyDay(key).total_tokens) }}</div><div class="mt-1 text-gray-500">{{ money(keyDay(key).actual_cost) }}</div></td>
                  <td class="px-3 py-3 text-right tabular-nums" :title="tokenBreakdown(key.total)"><div>{{ number(key.total.requests) }} / {{ number(key.total.total_tokens) }}</div><div class="mt-1 text-gray-500">{{ money(key.total.actual_cost) }}</div></td>
                  <td class="px-3 py-3 text-right text-gray-500">{{ dateTime(key.last_used_at) }}</td>
                </tr>
                <tr v-if="filteredKeys.length === 0"><td colspan="4" class="px-3 py-8 text-center text-gray-500">{{ t('admin.users.usageDetails.noKeys') }}</td></tr>
              </tbody>
            </table>
          </div>
          <div v-if="filteredKeys.length > pageSize" class="mt-2 flex items-center justify-end gap-3 text-xs">
            <button class="btn btn-secondary btn-sm" :disabled="page === 1" :aria-label="t('admin.users.usageDetails.previous')" @click="page--">‹</button>
            <span>{{ page }} / {{ pageCount }}</span>
            <button class="btn btn-secondary btn-sm" :disabled="page >= pageCount" :aria-label="t('admin.users.usageDetails.next')" @click="page++">›</button>
          </div>
        </div>
        <p class="text-xs text-gray-400">{{ t('admin.users.usageDetails.totalHint') }} · {{ t('admin.users.usageDetails.updatedAt') }} {{ dateTime(details.generated_at) }}</p>
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
const money = (value: number) => new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 6 }).format(value)
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
