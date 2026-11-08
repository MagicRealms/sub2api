<template>
  <div class="sunworld-chart-card sw-chart-panel" data-testid="usage-trends">
    <div class="sw-section-head flex flex-wrap">
      <h3><WorkspaceIcon name="analysis" />{{ title || t('admin.dashboard.tokenUsageTrend') }}</h3>
      <div class="sw-segmented" role="group" :aria-label="title || t('admin.dashboard.tokenUsageTrend')">
        <button v-for="item in chartMetrics" :key="item.value" type="button" :class="{ 'is-active': metric === item.value }" :aria-pressed="metric === item.value" @click="metric = item.value">{{ item.label }}</button>
      </div>
    </div>
    <div v-if="interactive" class="sw-trend-overview">
      <button v-for="item in overviewMetrics" :key="item.metric" type="button" :class="{ 'is-active': metric === item.metric }" @click="metric = item.metric">
        <span>{{ item.label }}</span><strong>{{ loading ? '—' : item.value }}</strong><small>{{ item.note }}</small>
      </button>
    </div>
    <div class="mb-3 flex flex-wrap items-baseline gap-2"><strong class="sw-chart-total">{{ summaryValue }}</strong><span class="sw-chart-sub">{{ metric === 'unit_cost' ? t('usage.unitCostPerMillion') : metric === 'cache_hit_rate' ? t('usage.cacheHitRate') : t('usage.inSelectedRange') }}</span><span v-if="interactive" class="ml-auto text-xs text-gray-400">{{ t('usage.chartPointHint') }}</span></div>
    <div v-if="loading" class="flex h-48 items-center justify-center"><LoadingSpinner /></div>
    <div v-else-if="trendData.length > 0 && chartData" :class="['h-48', { 'cursor-crosshair': interactive }]"><Bar v-if="displayMode === 'bar'" :data="barData" :options="barOptions" /><Line v-else :data="chartData" :options="lineOptions" /></div>
    <div v-else class="flex h-48 items-center justify-center text-xs text-gray-400">{{ t('admin.dashboard.noDataAvailable') }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutationObserver } from '@vueuse/core'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
import { formatTokenMillions } from '@/utils/workspace'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line, Bar } from 'vue-chartjs'
import type { ChartData, ChartOptions } from 'chart.js'
import { useChartTooltip } from '@/composables/useChartTooltip'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()
const externalTooltip = useChartTooltip()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
  title?: string
  displayMode?: 'line' | 'bar'
  interactive?: boolean
}>()
const emit = defineEmits<{ pointSelect: [point: TrendDataPoint] }>()

const isDarkMode = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(document.documentElement, () => { isDarkMode.value = document.documentElement.classList.contains('dark') }, { attributes: true, attributeFilter: ['class'] })
const metric = ref<'tokens' | 'requests' | 'cost' | 'cache_hit_rate' | 'unit_cost'>('tokens')
const chartMetrics = computed(() => [
  { value: 'tokens' as const, label: 'Token' },
  { value: 'requests' as const, label: t('admin.dashboard.requests') },
  { value: 'cost' as const, label: t('usage.cost') },
  { value: 'cache_hit_rate' as const, label: t('usage.cacheHitRate') },
  { value: 'unit_cost' as const, label: t('usage.unitCost') },
])
const totals = computed(() => props.trendData.reduce((sum, d) => ({
  tokens: sum.tokens + Number(d.total_tokens ?? (d.input_tokens + d.output_tokens + d.cache_creation_tokens + d.cache_read_tokens)),
  prompt: sum.prompt + Number(d.input_tokens || 0) + Number(d.cache_read_tokens || 0) + Number(d.cache_creation_tokens || 0),
  cache: sum.cache + Number(d.cache_read_tokens || 0),
  cost: sum.cost + Number(d.actual_cost || 0),
}), { tokens: 0, prompt: 0, cache: 0, cost: 0 }))
const overviewMetrics = computed(() => [
  { metric: 'cache_hit_rate' as const, label: t('usage.cacheHitRate'), value: `${(totals.value.prompt > 0 ? totals.value.cache / totals.value.prompt * 100 : 0).toFixed(1)}%`, note: t('usage.cacheHitRateBasis') },
  { metric: 'unit_cost' as const, label: t('usage.unitCost'), value: `$${(totals.value.tokens > 0 ? totals.value.cost / totals.value.tokens * 1_000_000 : 0).toFixed(2)}`, note: t('usage.unitCostPerMillion') },
  { metric: 'tokens' as const, label: t('admin.dashboard.totalTokens'), value: formatTokenMillions(totals.value.tokens), note: t('usage.inSelectedRange') },
])
const summaryValue = computed(() => {
  if (metric.value === 'requests') return props.trendData.reduce((sum, d) => sum + Number(d.requests || 0), 0).toLocaleString()
  if (metric.value === 'cost') return `$${props.trendData.reduce((sum, d) => sum + Number(d.actual_cost || 0), 0).toFixed(2)}`
  if (metric.value === 'cache_hit_rate') {
    const cacheRead = props.trendData.reduce((sum, d) => sum + Number(d.cache_read_tokens || 0), 0)
    const promptTokens = props.trendData.reduce((sum, d) => sum + Number(d.input_tokens || 0) + Number(d.cache_read_tokens || 0) + Number(d.cache_creation_tokens || 0), 0)
    return `${promptTokens > 0 ? ((cacheRead / promptTokens) * 100).toFixed(1) : '0.0'}%`
  }
  if (metric.value === 'unit_cost') {
    const tokens = props.trendData.reduce((sum, d) => sum + Number(d.total_tokens || 0), 0)
    const cost = props.trendData.reduce((sum, d) => sum + Number(d.actual_cost || 0), 0)
    return `$${(tokens > 0 ? (cost / tokens) * 1_000_000 : 0).toFixed(2)}`
  }
  return formatTokenMillions(props.trendData.reduce((sum, d) => sum + d.input_tokens + d.output_tokens + d.cache_creation_tokens + d.cache_read_tokens, 0))
})

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#93a4bd' : '#8794a9',
  grid: isDarkMode.value ? 'rgba(143,172,211,.09)' : 'rgba(138,163,193,.1)',
  input: '#6c9ee5',
  output: '#69c4b1',
  cacheCreation: '#e8be7e',
  cacheRead: '#a498d9',
  cacheHitRate: '#91a1b7'
}))

const chartData = computed(() => {
  if (!props.trendData?.length) return null
  if (metric.value !== 'tokens') {
    const isRate = metric.value === 'cache_hit_rate'
    const isUnitCost = metric.value === 'unit_cost'
    const values = props.trendData.map((d) => {
      const tokens = Number(d.total_tokens || 0)
      const promptTokens = Number(d.input_tokens || 0) + Number(d.cache_read_tokens || 0) + Number(d.cache_creation_tokens || 0)
      if (isRate) return promptTokens > 0 ? (Number(d.cache_read_tokens || 0) / promptTokens) * 100 : 0
      if (isUnitCost) return tokens > 0 ? (Number(d.actual_cost || 0) / tokens) * 1_000_000 : 0
      return metric.value === 'cost' ? Number(d.actual_cost || 0) : Number(d.requests || 0)
    })
    const label = isRate ? t('usage.cacheHitRate') : isUnitCost ? `${t('usage.unitCost')} (${t('usage.unitCostPerMillion')})` : metric.value === 'cost' ? t('usage.cost') : t('admin.dashboard.requests')
    const color = isRate ? chartColors.value.cacheHitRate : chartColors.value.input
    return {
      labels: props.trendData.map(d => d.date),
      datasets: [{ label, data: values, borderColor: color, backgroundColor: `${color}16`, borderWidth: 2, pointRadius: props.interactive ? 3 : 0, pointHoverRadius: 5, pointHitRadius: props.interactive ? 10 : 0, tension: .4, fill: true, ...(isRate ? { yAxisID: 'yPercent' } : {}) }]
    }
  }
  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: 'Input',
        data: props.trendData.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: `${chartColors.value.input}18`,
        pointRadius: props.interactive ? 2 : 0,
        pointHoverRadius: 4,
        pointHitRadius: props.interactive ? 10 : 0,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Output',
        data: props.trendData.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}16`,
        pointRadius: props.interactive ? 2 : 0,
        pointHoverRadius: 4,
        pointHitRadius: props.interactive ? 10 : 0,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Cache Creation',
        data: props.trendData.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: `${chartColors.value.cacheCreation}14`,
        pointRadius: props.interactive ? 2 : 0,
        pointHitRadius: props.interactive ? 10 : 0,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Cache Read',
        data: props.trendData.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: `${chartColors.value.cacheRead}14`,
        pointRadius: props.interactive ? 2 : 0,
        pointHitRadius: props.interactive ? 10 : 0,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Cache Hit Rate',
        data: props.trendData.map((d) => {
          const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
          return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
        }),
        borderColor: chartColors.value.cacheHitRate,
        backgroundColor: `${chartColors.value.cacheHitRate}14`,
        pointRadius: props.interactive ? 2 : 0,
        pointHitRadius: props.interactive ? 10 : 0,
        borderDash: [5, 5],
        fill: false,
        tension: 0.4,
        yAxisID: 'yPercent'
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: { duration: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : 600 },
  elements: { line: { borderWidth: 2 } },
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  onClick: (_event: unknown, elements: { index: number }[]) => {
    const index = elements[0]?.index
    const point = index === undefined ? undefined : props.trendData[index]
    if (props.interactive && point) emit('pointSelect', point)
  },
  plugins: {
    legend: {
      position: 'bottom' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 5,
        boxHeight: 5,
        padding: 12,
        font: {
          size: 9
        }
      }
    },
    tooltip: {
      enabled: false,
      external: externalTooltip,
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          return `${context.dataset.label}: ${metric.value === 'cost' ? '$' + Number(context.raw).toFixed(4) : metric.value === 'requests' ? Number(context.raw).toLocaleString() : metric.value === 'cache_hit_rate' ? `${Number(context.raw).toFixed(1)}%` : metric.value === 'unit_cost' ? '$' + Number(context.raw).toFixed(4) : formatTokens(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && props.trendData[dataIndex]) {
            const data = props.trendData[dataIndex]
            return `Actual: $${formatCost(data.actual_cost)} | Standard: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid,
        drawBorder: false
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid,
        drawBorder: false
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => metric.value === 'cost' || metric.value === 'unit_cost' ? '$' + Number(value).toFixed(2) : metric.value === 'requests' ? Number(value).toLocaleString() : metric.value === 'cache_hit_rate' ? `${value}%` : formatTokens(Number(value))
      }
    },
    yPercent: {
      display: metric.value === 'tokens' || metric.value === 'cache_hit_rate',
      position: 'right' as const,
      min: 0,
      max: 100,
      grid: {
        drawOnChartArea: false
      },
      ticks: {
        color: chartColors.value.cacheHitRate,
        font: {
          size: 10
        },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))

const barData = computed<ChartData<'bar'>>(() => ({
  labels: chartData.value?.labels ?? [],
  datasets: (chartData.value?.datasets ?? []).filter(dataset => metric.value !== 'tokens' || dataset.yAxisID !== 'yPercent').map(dataset => ({
    label: dataset.label, data: dataset.data, yAxisID: dataset.yAxisID,
    backgroundColor: dataset.borderColor, borderRadius: 5, borderSkipped: false, maxBarThickness: 24, categoryPercentage: .65,
  })),
}))
const barOptions = computed<ChartOptions<'bar'>>(() => ({
  ...lineOptions.value,
  scales: { ...lineOptions.value.scales, y: { ...lineOptions.value.scales.y, beginAtZero: true }, yPercent: { ...lineOptions.value.scales.yPercent, display: metric.value === 'cache_hit_rate' } },
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}
</script>
