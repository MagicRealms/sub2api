<template>
  <div class="space-y-3" data-test="ordered-group-selector">
    <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('keys.routing.hint') }}</p>
    <ol v-if="modelValue.length" class="space-y-1.5" aria-live="polite">
      <li v-for="(id, index) in modelValue" :key="id" class="flex min-w-0 items-center gap-2 rounded-xl border border-primary-200 bg-primary-50/70 p-2 dark:border-primary-800 dark:bg-primary-900/20" :data-route-id="id">
        <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-600 text-xs font-semibold text-white" :aria-label="t('keys.routing.priority', { order: index + 1 })">{{ index + 1 }}</span>
        <span class="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-white" :title="findOption(id)?.label ?? `#${id}`">{{ findOption(id)?.label ?? `#${id}` }}</span>
        <span class="shrink-0 text-xs text-gray-500">{{ findOption(id)?.userRate ?? findOption(id)?.rate ?? '—' }}×</span>
        <button type="button" class="rounded-lg p-1 text-gray-500 transition hover:bg-white/70 hover:text-primary-600 disabled:opacity-30 dark:hover:bg-dark-800" :disabled="disabled || index === 0" :aria-label="t('keys.routing.moveUp')" @click="move(index, -1)">↑</button>
        <button type="button" class="rounded-lg p-1 text-gray-500 transition hover:bg-white/70 hover:text-primary-600 disabled:opacity-30 dark:hover:bg-dark-800" :disabled="disabled || index === modelValue.length - 1" :aria-label="t('keys.routing.moveDown')" @click="move(index, 1)">↓</button>
        <button type="button" class="rounded-lg p-1 text-gray-500 transition hover:bg-white/70 hover:text-coral-600" :disabled="disabled" :aria-label="t('common.remove')" @click="toggle(id)">×</button>
      </li>
    </ol>
    <div class="max-h-60 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-1.5 dark:border-dark-600">
      <label v-for="option in options" :key="option.value" class="flex cursor-pointer items-start gap-2 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
        <input type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600" :checked="modelValue.includes(option.value)" :disabled="disabled || (!modelValue.includes(option.value) && modelValue.length >= 16)" @change="toggle(option.value)" />
        <GroupOptionItem :name="option.label" :platform="option.platform" :subscription-type="option.subscriptionType" :rate-multiplier="option.rate" :user-rate-multiplier="option.userRate" :description="option.description" :show-checkmark="false" :peak-rate-enabled="option.peakRateEnabled" :peak-start="option.peakStart" :peak-end="option.peakEnd" :peak-rate-multiplier="option.peakRateMultiplier" />
        <span v-if="modelValue.includes(option.value)" class="shrink-0 rounded-full bg-primary-100 px-2 py-0.5 text-xs font-bold text-primary-700 dark:bg-primary-900 dark:text-primary-200">#{{ modelValue.indexOf(option.value) + 1 }}</span>
      </label>
      <p v-if="!options.length" class="p-3 text-center text-sm text-gray-400">{{ t('common.noGroupsAvailable') }}</p>
    </div>
    <p class="rounded-lg bg-amber-50 p-2.5 text-xs leading-5 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t('keys.routing.billingHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type { GroupPlatform, SubscriptionType } from '@/types'

export interface OrderedGroupOption {
  value: number
  label: string
  platform: GroupPlatform
  description?: string | null
  rate?: number
  userRate?: number | null
  subscriptionType?: SubscriptionType
  peakRateEnabled?: boolean
  peakStart?: string
  peakEnd?: string
  peakRateMultiplier?: number
}
const props = defineProps<{ modelValue: number[]; options: OrderedGroupOption[]; allOptions?: OrderedGroupOption[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const { t } = useI18n()
const findOption = (id: number) => (props.allOptions ?? props.options).find(option => option.value === id)
const toggle = (id: number) => {
  if (props.disabled) return
  if (props.modelValue.includes(id)) emit('update:modelValue', props.modelValue.filter(value => value !== id))
  else if (props.modelValue.length < 16) emit('update:modelValue', [...props.modelValue, id])
}
const move = (index: number, direction: number) => {
  if (props.disabled || index + direction < 0 || index + direction >= props.modelValue.length) return
  const next = [...props.modelValue]
  ;[next[index], next[index + direction]] = [next[index + direction]!, next[index]!]
  emit('update:modelValue', next)
}
</script>
