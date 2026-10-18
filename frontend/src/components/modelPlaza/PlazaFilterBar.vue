<template>
  <div class="space-y-3 rounded-2xl border border-gray-200/70 bg-white/60 p-4 dark:border-dark-700/70 dark:bg-dark-800/40">
    <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
      <div class="relative min-w-0 flex-1 sm:min-w-[200px]">
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500" />
        <input
          :value="search"
          type="text"
          :aria-label="t('modelPlaza.filters.searchPlaceholder')"
          :placeholder="t('modelPlaza.filters.searchPlaceholder')"
          class="input w-full rounded-xl py-2 pl-9 pr-9 text-sm dark:bg-dark-900 dark:text-gray-100"
          @input="$emit('update:search', ($event.target as HTMLInputElement).value)"
        />
        <button v-if="search" type="button" :aria-label="t('modelPlaza.cards.clearSearch')" class="absolute right-2.5 top-1/2 -translate-y-1/2 rounded text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="$emit('update:search', '')"><Icon name="x" size="sm" /></button>
      </div>
      <div class="grid min-w-0 grid-cols-2 gap-3 sm:flex">
        <select :value="groupId" :aria-label="t('modelPlaza.filters.groupLabel')" class="input min-w-0 rounded-xl py-2 text-sm dark:bg-dark-900 dark:text-gray-100 sm:max-w-[220px]" @change="$emit('update:groupId', ($event.target as HTMLSelectElement).value === 'all' ? 'all' : Number(($event.target as HTMLSelectElement).value))">
          <option value="all">{{ t('modelPlaza.cards.allGroups') }}</option>
          <option v-for="g in groups" :key="g.id" :value="g.id" :disabled="!groupEnabled(g)">{{ g.name }}</option>
        </select>
        <select :value="rate" :aria-label="t('modelPlaza.filters.rateLabel')" class="input min-w-0 rounded-xl py-2 text-sm dark:bg-dark-900 dark:text-gray-100 sm:w-32" @change="$emit('update:rate', ($event.target as HTMLSelectElement).value === 'all' ? 'all' : Number(($event.target as HTMLSelectElement).value))">
          <option value="all">{{ t('modelPlaza.cards.allRates') }}</option>
          <option v-for="r in rates" :key="r" :value="r" :disabled="!rateEnabled(r)">{{ r }}×</option>
        </select>
      </div>
    </div>
    <div v-if="platforms.length" class="flex flex-wrap gap-2">
      <button
        v-for="p in ['all', ...platforms]"
        :key="p"
        type="button"
        class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-40 disabled:grayscale"
        :class="p === 'all' ? chipClass(platform === 'all') : platform === p ? 'chip-tinted-active' : 'chip-tinted'"
        :style="p === 'all' ? undefined : { '--chip-accent': platformAccentColor(p) }"
        :disabled="p !== 'all' && !platformEnabled(p)"
        :aria-pressed="platform === p"
        @click="$emit('update:platform', p)"
      >
        <PlatformIcon v-if="p !== 'all'" :platform="p as GroupPlatform" size="xs" />
        {{ p === 'all' ? t('modelPlaza.filters.all') : platformLabel(p) }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformAccentColor, platformLabel } from '@/utils/platformColors'
import type { GroupPlatform } from '@/types'

const props = defineProps<{
  /** 数据中出现的平台(去重排序后)。 */
  platforms: string[]
  /** 全量分组(含平台与生效倍率),三个维度的置灰联动由此推导。 */
  groups: Array<{ id: number; name: string; platform: string; rate: number }>
  /** 全量生效倍率去重升序。 */
  rates: number[]
  platform: string
  groupId: number | 'all'
  rate: number | 'all'
  /** 模型名搜索词(纯前端过滤)。 */
  search: string
}>()

defineEmits<{
  'update:platform': [value: string]
  'update:groupId': [value: number | 'all']
  'update:rate': [value: number | 'all']
  'update:search': [value: string]
}>()

const { t } = useI18n()

/**
 * 三个维度互为约束(faceted):某选项可点 ⟺ 在「其他两维」当前选择下仍有分组命中。
 * 「全部」永远可点,作为解除本维约束的出口;可点项组合恒有结果,无需选择修正。
 */
function platformEnabled(p: string): boolean {
  return props.groups.some(
    (g) =>
      g.platform === p &&
      (props.groupId === 'all' || g.id === props.groupId) &&
      (props.rate === 'all' || g.rate === props.rate)
  )
}

function groupEnabled(g: { platform: string; rate: number }): boolean {
  return (
    (props.platform === 'all' || g.platform === props.platform) &&
    (props.rate === 'all' || g.rate === props.rate)
  )
}

function rateEnabled(r: number): boolean {
  return props.groups.some(
    (g) =>
      g.rate === r &&
      (props.platform === 'all' || g.platform === props.platform) &&
      (props.groupId === 'all' || g.id === props.groupId)
  )
}

function chipClass(active: boolean): string {
  return active
    ? 'bg-gradient-to-r from-primary-500 to-primary-600 text-white shadow-sm shadow-primary-500/30'
    : 'bg-white text-gray-600 ring-1 ring-inset ring-gray-200 enabled:hover:bg-gray-50 enabled:hover:text-gray-900 enabled:hover:ring-gray-300 dark:bg-dark-800/60 dark:text-dark-300 dark:ring-dark-700 dark:enabled:hover:bg-dark-800 dark:enabled:hover:text-white'
}
</script>

<style scoped>
/* 平台/分组 chip 的配色统一从 --chip-accent(平台主色)派生,新增平台无需扩展样式。
   激活态与非激活态在模板上互斥挂载,避免选择器优先级互相覆盖。 */
.chip-tinted {
  color: var(--chip-accent);
  color: color-mix(in srgb, var(--chip-accent) 78%, black);
  background-color: color-mix(in srgb, var(--chip-accent) 9%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 25%, transparent);
}

.chip-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 16%, transparent);
}

.dark .chip-tinted {
  color: color-mix(in srgb, var(--chip-accent) 72%, white);
  background-color: color-mix(in srgb, var(--chip-accent) 12%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 30%, transparent);
}

.dark .chip-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 18%, transparent);
}

.chip-tinted-active {
  color: #fff;
  background-color: var(--chip-accent);
  background-color: color-mix(in srgb, var(--chip-accent) 85%, black);
  box-shadow: 0 1px 2px 0 color-mix(in srgb, var(--chip-accent) 35%, transparent);
}

.chip-tinted-active:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 75%, black);
}

.dark .chip-tinted-active {
  background-color: color-mix(in srgb, var(--chip-accent) 80%, transparent);
}

.dark .chip-tinted-active:not(:disabled):hover {
  background-color: var(--chip-accent);
}
</style>
