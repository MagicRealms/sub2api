<template>
  <div class="sw-dashboard-stats space-y-3">
    <div class="sw-metric-grid" :class="{ 'sw-simple-metrics': isSimple }">
      <MetricCard v-if="!isSimple" :label="t('dashboard.balance')" :value="`$${formatBalance(balance)}`" :detail="t('common.available')" icon="wallet" tone="mint" />
      <MetricCard :label="t('dashboard.todayRequests')" :value="number(stats.today_requests)" icon="pulse" tone="sky">
        {{ t('common.total') }} <b>{{ number(stats.total_requests) }}</b>
      </MetricCard>
      <MetricCard :label="t('dashboard.todayTokens')" :value="tokens(stats.today_tokens)" icon="models" tone="violet">
        {{ t('dashboard.totalTokens') }} <b>{{ tokens(stats.total_tokens) }}</b>
      </MetricCard>
      <MetricCard :label="t('dashboard.todayCost')" :value="`$${cost(stats.today_actual_cost)}`" icon="wallet" tone="sun">
        {{ t('common.total') }} <b>${{ cost(stats.total_actual_cost) }}</b>
      </MetricCard>
    </div>
    <div class="sw-secondary-metrics">
      <div><span>{{ t('dashboard.apiKeys') }}</span><b>{{ stats.active_api_keys }} / {{ stats.total_api_keys }}</b></div>
      <div><span>{{ t('dashboard.cacheToday') }}</span><b>{{ tokens(stats.today_cache_read_tokens + stats.today_cache_creation_tokens) }}</b></div>
      <div><span>{{ t('dashboard.avgResponse') }}</span><b>{{ duration(stats.average_duration_ms) }}</b></div>
      <div><span>RPM</span><b>{{ Number(stats.rpm || 0).toFixed(1) }}</b></div>
      <div><span>TPM</span><b>{{ number(Math.round(stats.tpm || 0)) }}</b></div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import MetricCard from '@/components/workspace/MetricCard.vue'
import type { UserDashboardStats } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'
import { formatTokenMillions as tokens } from '@/utils/workspace'
defineProps<{ stats: UserDashboardStats; balance: number; isSimple: boolean; platformQuotas?: PlatformQuotaItem[] | null }>()
const { t } = useI18n()
const number = (value: number) => Number(value || 0).toLocaleString()
const cost = (value: number) => Number(value || 0).toFixed(2)
const formatBalance = (value: number) => Number(value || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const duration = (value: number) => value >= 1000 ? `${(value / 1000).toFixed(1)} s` : `${Math.round(value || 0)} ms`
</script>
