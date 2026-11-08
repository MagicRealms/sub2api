<template>
  <div class="space-y-4">
    <div class="sw-filterbar">
      <span>{{ t('dashboard.timeRange') }}</span>
      <DateRangePicker :time-zone="DASHBOARD_TIMEZONE" :start-date="startDate" :end-date="endDate" @update:startDate="$emit('update:startDate', $event)" @update:endDate="$emit('update:endDate', $event)" @change="$emit('dateRangeChange', $event)" />
      <div class="sw-filter-end">
        <div class="sw-segmented">
          <button v-for="unit in ['hour','day','week']" :key="unit" :class="{ 'is-active': granularity === unit }" :aria-pressed="granularity === unit" @click="$emit('update:granularity', unit); $emit('granularityChange')">{{ t(`dashboard.${unit}`) }}</button>
        </div>
        <ChartDisplayToggle v-model="displayMode" />
        <button :disabled="loading" class="btn btn-secondary" :aria-label="t('common.refresh')" @click="$emit('refresh')"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" /></button>
      </div>
    </div>
    <div class="sw-chart-grid">
      <ModelDistributionChart :display-mode="displayMode" :title="t('dashboard.consumptionDistribution')" :model-stats="models" :loading="loading" v-model:metric="metric" show-metric-toggle :enable-breakdown="false" :show-account-cost="false" compact />
      <TokenUsageTrend :display-mode="displayMode" :trend-data="trend" :loading="loading" :title="t('dashboard.modelCallAnalysis')" />
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import ChartDisplayToggle from '@/components/charts/ChartDisplayToggle.vue'
import { DASHBOARD_TIMEZONE } from '@/utils/dashboardTime'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { TrendDataPoint, ModelStat } from '@/types'
defineProps<{ loading: boolean; startDate: string; endDate: string; granularity: string; trend: TrendDataPoint[]; models: ModelStat[] }>()
defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'granularityChange', 'dateRangeChange', 'refresh'])
const { t } = useI18n()
const displayMode = ref<'line' | 'bar'>('line')
const metric = ref<'tokens' | 'actual_cost'>('actual_cost')
</script>
