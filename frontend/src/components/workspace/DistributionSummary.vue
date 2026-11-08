<template>
  <div class="sw-distribution">
    <div class="sw-donut sw-native-donut" :style="{ background: gradient }" aria-hidden="true">
      <div class="sw-donut-center"><strong>{{ format(total) }}</strong><span>{{ t(metric === 'tokens' ? 'usage.totalTokens' : 'usage.totalCost') }}</span></div>
    </div>
    <div class="sw-model-list">
      <div v-for="(item, index) in items" :key="item.key">
        <button type="button" class="sw-model-list-row w-full text-left" :disabled="!interactive" @click="emit('select', item.key)">
          <i class="sw-model-list-dot" :style="{ background: color(index) }" />
          <span class="sw-model-list-name" :title="item.label">{{ item.label }}</span><span class="sw-model-list-value">{{ format(item.value) }}</span>
          <span class="sw-model-list-track"><i :style="{ background: color(index), width: `${total > 0 ? Math.max(0, item.value) / total * 100 : 0}%` }" /></span>
        </button>
        <slot :item="item" />
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatTokenMillions, workspacePalette } from '@/utils/workspace'
const props = defineProps<{ items: { key: string | number; label: string; value: number }[]; metric: 'tokens' | 'actual_cost'; interactive?: boolean }>()
const emit = defineEmits<{ select: [key: string | number] }>()
const { t } = useI18n()
const total = computed(() => props.items.reduce((sum, item) => sum + Math.max(0, item.value || 0), 0))
const color = (index: number) => workspacePalette[index % workspacePalette.length]
const format = (value: number) => props.metric === 'tokens' ? formatTokenMillions(value) : `$${value.toFixed(2)}`
const gradient = computed(() => {
  if (!total.value) return 'var(--sw-line)'
  let offset = 0
  const stops = props.items.map((item, i) => { const start = offset; offset += Math.max(0, item.value || 0) / total.value * 100; return `${color(i)} ${start}% ${offset}%` })
  return `conic-gradient(${stops.join(',')})`
})
</script>
