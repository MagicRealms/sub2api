<template>
  <div class="usage-insight-grid grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4" data-testid="usage-insights">
    <button v-for="card in distributions" :key="card.key" type="button" class="insight-card" :data-insight="card.key" aria-haspopup="dialog" @click="emit('open', card.key)">
      <span class="insight-heading"><span class="insight-icon" :style="{ color: card.color }"><WorkspaceIcon :name="card.icon" /></span><span>{{ card.title }}</span><Icon name="arrowRight" size="sm" class="ml-auto shrink-0 text-gray-400" /></span>
      <strong class="insight-value">{{ card.loading ? '—' : card.value }}</strong>
      <span class="insight-description truncate">{{ card.loading ? t('common.loading') : card.leader || t('workspace.noActivity') }}</span>
      <span class="insight-bar" aria-hidden="true"><span v-for="(item, index) in card.segments" :key="index" :style="{ width: `${item}%`, background: workspacePalette[index % workspacePalette.length] }" /></span>
      <span class="insight-footer"><span>{{ card.loading ? '—' : card.note }}</span><span>{{ t('workspace.insightDetails') }}<Icon name="arrowRight" size="xs" /></span></span>
    </button>
    <button type="button" class="insight-card insight-trends" data-insight="trends" aria-haspopup="dialog" @click="emit('open', 'trends')">
      <span class="insight-heading"><span class="insight-icon text-violet-500"><WorkspaceIcon name="analysis" /></span><span>{{ t('workspace.trendSnapshot') }}</span><Icon name="arrowRight" size="sm" class="ml-auto shrink-0 text-gray-400" /></span>
      <span class="flex items-baseline gap-2"><strong class="insight-value">{{ distributionLoading ? '—' : `${cacheHitRate.toFixed(1)}%` }}</strong><span class="text-[10px] text-gray-400">{{ t('usage.cacheHitRate') }}</span></span>
      <span class="insight-description flex items-center gap-2"><span>{{ t('usage.unitCost') }}</span><strong class="font-medium text-gray-700 dark:text-gray-300">{{ distributionLoading ? '—' : `$${unitCost.toFixed(2)}/M` }}</strong><span class="ml-auto">{{ formatTokenMillions(totals.tokens) }}</span></span>
      <svg class="insight-spark" viewBox="0 0 240 20" preserveAspectRatio="none" aria-hidden="true"><path v-if="sparkPath && !distributionLoading" :d="sparkPath" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke" /></svg>
      <span class="insight-footer"><span>{{ t('usage.inSelectedRange') }}</span><span>{{ t('workspace.insightDetails') }}<Icon name="arrowRight" size="xs" /></span></span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
import { formatTokenMillions, workspacePalette } from '@/utils/workspace'
import type { ModelStat, GroupStat, EndpointStat, TrendDataPoint } from '@/types'

export type UsageInsight = 'models' | 'groups' | 'endpoints' | 'trends'
const props = defineProps<{
  modelStats: ModelStat[]
  groupStats: GroupStat[]
  endpointStats: EndpointStat[]
  trendData: TrendDataPoint[]
  modelLoading?: boolean
  distributionLoading?: boolean
  endpointLoading?: boolean
}>()
const emit = defineEmits<{ open: [insight: UsageInsight] }>()
const { t } = useI18n()
const number = (value: unknown) => Math.max(0, Number(value) || 0)
const totals = computed(() => props.trendData.reduce((sum, row) => ({
  tokens: sum.tokens + number(row.total_tokens ?? (number(row.input_tokens) + number(row.output_tokens) + number(row.cache_read_tokens) + number(row.cache_creation_tokens))),
  prompt: sum.prompt + number(row.input_tokens) + number(row.cache_read_tokens) + number(row.cache_creation_tokens),
  cache: sum.cache + number(row.cache_read_tokens),
  cost: sum.cost + number(row.actual_cost),
}), { tokens: 0, prompt: 0, cache: 0, cost: 0 }))
const cacheHitRate = computed(() => totals.value.prompt > 0 ? totals.value.cache / totals.value.prompt * 100 : 0)
const unitCost = computed(() => totals.value.tokens > 0 ? totals.value.cost / totals.value.tokens * 1_000_000 : 0)

function distribution(key: Exclude<UsageInsight, 'trends'>, rows: { label: string; value: number }[], loading: boolean | undefined, title: string, icon: string, color: string, value: string, note: string) {
  const sorted = [...rows].sort((a, b) => b.value - a.value)
  const total = sorted.reduce((sum, row) => sum + row.value, 0)
  const share = total > 0 ? sorted[0]!.value / total * 100 : 0
  const top = sorted.slice(0, 3)
  const rest = sorted.slice(3).reduce((sum, row) => sum + row.value, 0)
  return { key, loading, title, icon, color, value, note, leader: sorted.length ? (total > 0 ? `${sorted[0]!.label} · ${t('workspace.leadingShare', { value: share.toFixed(0) })}` : sorted[0]!.label) : '', segments: total > 0 ? [...top.map(row => row.value), rest].filter(v => v > 0).map(v => v / total * 100) : [] }
}
const distributions = computed(() => [
  distribution('models', props.modelStats.map(row => ({ label: row.model, value: number(row.actual_cost) })), props.modelLoading, t('admin.dashboard.consumptionDistribution'), 'models', '#6c9ee5', `$${props.modelStats.reduce((sum, row) => sum + number(row.actual_cost), 0).toFixed(2)}`, `${props.modelStats.length} ${t('workspace.modelsUsed')}`),
  distribution('groups', props.groupStats.map(row => ({ label: row.group_name || t('admin.dashboard.noGroup'), value: number(row.total_tokens) })), props.distributionLoading, t('admin.dashboard.groupDistribution'), 'group', '#69c4b1', formatTokenMillions(props.groupStats.reduce((sum, row) => sum + number(row.total_tokens), 0)), `${props.groupStats.length} ${t('workspace.groupsUsed')}`),
  distribution('endpoints', props.endpointStats.map(row => ({ label: row.endpoint, value: number(row.requests) })), props.endpointLoading, t('usage.endpointDistribution'), 'channels', '#e8be7e', `${props.endpointStats.reduce((sum, row) => sum + number(row.requests), 0).toLocaleString()} ${t('admin.dashboard.requests')}`, `${props.endpointStats.length} ${t('workspace.endpointsUsed')}`),
])
const sparkPath = computed(() => {
  const values = props.trendData.map(row => number(row.total_tokens ?? (number(row.input_tokens) + number(row.output_tokens) + number(row.cache_read_tokens) + number(row.cache_creation_tokens))))
  if (!values.length) return ''
  if (values.length === 1) values.push(values[0]!)
  const max = Math.max(...values, 1)
  return values.map((value, index) => `${index ? 'L' : 'M'}${(index / (values.length - 1) * 238 + 1).toFixed(1)},${(18 - value / max * 16).toFixed(1)}`).join(' ')
})
</script>

<style scoped>
.insight-card { display: flex; min-width: 0; flex-direction: column; gap: .5rem; padding: 1rem; text-align: left; border: 1px solid #e1ebe8; border-radius: 1.2rem; background: linear-gradient(145deg, #fff, #f8fbfa); transition: border-color .2s, box-shadow .2s, transform .2s; }
.insight-card:hover { border-color: #9ecbbc; box-shadow: 0 6px 22px #4f8c7710; transform: translateY(-2px); }
.insight-card:focus-visible { outline: 2px solid #72b8a3; outline-offset: 3px; }
.insight-heading { display: flex; align-items: center; gap: .5rem; min-height: 1.75rem; font-size: .72rem; font-weight: 600; color: #52667a; }
.insight-icon { display: grid; height: 1.7rem; width: 1.7rem; flex-shrink: 0; place-items: center; border-radius: .55rem; background: color-mix(in srgb, currentColor 9%, transparent); }
.insight-icon svg { width: 1rem; height: 1rem; }
.insight-value { margin-top: .15rem; font-size: 1.55rem; font-weight: 650; letter-spacing: -.04em; line-height: 1.3; color: #263a4c; font-variant-numeric: tabular-nums; }
.insight-description { min-height: 1.1rem; font-size: .67rem; color: #8191a1; }
.insight-bar { display: flex; height: 5px; gap: 3px; margin-block: .45rem; overflow: hidden; border-radius: 5px; background: #edf3f1; }
.insight-bar > span { height: 100%; border-radius: 5px; }
.insight-spark { height: 19px; width: 100%; margin-top: -.1rem; color: #a498d9; }
.insight-footer { display: flex; justify-content: space-between; align-items: center; gap: .5rem; margin-top: auto; padding-top: .35rem; font-size: .6rem; color: #8a9b9e; }
.insight-footer > span:last-child { display: flex; align-items: center; gap: .15rem; color: #6f948b; }
.dark .insight-card { border-color: #293d45; background: linear-gradient(145deg, #1b2b34, #172630); }
.dark .insight-card:hover { border-color: #507b70; }
.dark .insight-heading { color: #a8bec9; }
.dark .insight-value { color: #e6f0ef; }
.dark .insight-description { color: #a0b2bf; }
.dark .insight-footer { color: #8fa6af; }
.dark .insight-footer > span:last-child { color: #8cc7b3; }
.dark .insight-bar { background: #263a41; }
@media (prefers-reduced-motion: reduce) { .insight-card { transition: none; } .insight-card:hover { transform: none; } }
</style>
