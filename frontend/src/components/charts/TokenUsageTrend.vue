<template>
  <div class="sunworld-chart-card sw-chart-panel">
    <div class="sw-section-head flex flex-wrap">
      <h3><WorkspaceIcon name="analysis" />{{ title || t('admin.dashboard.tokenUsageTrend') }}</h3>
      <div class="sw-segmented" role="group" :aria-label="title || t('admin.dashboard.tokenUsageTrend')">
        <button v-for="item in chartMetrics" :key="item.value" type="button" :class="{ 'is-active': metric === item.value }" :aria-pressed="metric === item.value" @click="metric = item.value">{{ item.label }}</button>
      </div>
    </div>
    <div class="mb-3 flex items-baseline gap-2"><strong class="sw-chart-total">{{ summaryValue }}</strong><span class="sw-chart-sub">{{ t('usage.inSelectedRange') }}</span></div>
    <div v-if="loading" class="flex h-48 items-center justify-center"><LoadingSpinner /></div>
    <div v-else-if="trendData.length > 0 && chartData" class="h-48"><Line :data="chartData" :options="lineOptions" /></div>
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
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
  title?: string
}>()

const isDarkMode = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(document.documentElement, () => { isDarkMode.value = document.documentElement.classList.contains('dark') }, { attributes: true, attributeFilter: ['class'] })
const metric = ref<'tokens' | 'requests' | 'cost'>('tokens')
const chartMetrics = computed(() => [
  { value: 'tokens' as const, label: 'Token' },
  { value: 'requests' as const, label: t('admin.dashboard.requests') },
  { value: 'cost' as const, label: t('usage.cost') },
])
const summaryValue = computed(() => {
  if (metric.value === 'requests') return props.trendData.reduce((sum, d) => sum + Number(d.requests || 0), 0).toLocaleString()
  if (metric.value === 'cost') return `$${props.trendData.reduce((sum, d) => sum + Number(d.actual_cost || 0), 0).toFixed(2)}`
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
  if (metric.value !== 'tokens') return {
    labels: props.trendData.map(d => d.date),
    datasets: [{ label: metric.value === 'cost' ? t('usage.cost') : t('admin.dashboard.requests'), data: props.trendData.map(d => metric.value === 'cost' ? Number(d.actual_cost || 0) : Number(d.requests || 0)), borderColor: chartColors.value.input, backgroundColor: `${chartColors.value.input}16`, borderWidth: 2, pointRadius: 0, pointHoverRadius: 4, tension: .4, fill: true }]
  }
  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: 'Input',
        data: props.trendData.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: `${chartColors.value.input}18`,
        pointRadius: 0,
        pointHoverRadius: 4,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Output',
        data: props.trendData.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}16`,
        pointRadius: 0,
        pointHoverRadius: 4,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Cache Creation',
        data: props.trendData.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: `${chartColors.value.cacheCreation}14`,
        pointRadius: 0,
        fill: true,
        tension: 0.4
      },
      {
        label: 'Cache Read',
        data: props.trendData.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: `${chartColors.value.cacheRead}14`,
        pointRadius: 0,
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
        pointRadius: 0,
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
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          return `${context.dataset.label}: ${metric.value === 'cost' ? '$' + Number(context.raw).toFixed(4) : metric.value === 'requests' ? Number(context.raw).toLocaleString() : formatTokens(context.raw)}`
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
        callback: (value: string | number) => metric.value === 'cost' ? '$' + Number(value).toFixed(2) : metric.value === 'requests' ? Number(value).toLocaleString() : formatTokens(Number(value))
      }
    },
    yPercent: {
      display: metric.value === 'tokens',
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
