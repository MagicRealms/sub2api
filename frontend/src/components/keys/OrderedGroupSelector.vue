<template>
  <div class="space-y-4" data-test="ordered-group-selector">
    <section aria-labelledby="selected-groups-heading">
      <div class="mb-2 flex items-center justify-between gap-3">
        <h3 id="selected-groups-heading" class="text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('keys.routing.selectedGroups') }}
        </h3>
        <span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ modelValue.length }}/16</span>
      </div>

      <ol v-if="modelValue.length" class="space-y-1.5" aria-live="polite">
        <li
          v-for="(id, index) in modelValue"
          :key="id"
          class="flex min-w-0 items-center gap-2 rounded-lg border border-primary-200/80 bg-primary-50/50 px-2 py-1.5 dark:border-primary-800/70 dark:bg-primary-950/30"
          :data-route-id="id"
        >
          <span
            class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-600 text-xs font-semibold text-white"
            :aria-label="t('keys.routing.priority', { order: index + 1 })"
          >{{ index + 1 }}</span>
          <GroupBadge
            class="min-w-0 flex-1"
            :name="findOption(id)?.label ?? `#${id}`"
            :platform="findOption(id)?.platform"
            :subscription-type="findOption(id)?.subscriptionType"
            :rate-multiplier="findOption(id)?.rate"
            :user-rate-multiplier="findOption(id)?.userRate"
            :peak-rate-enabled="findOption(id)?.peakRateEnabled"
            :peak-start="findOption(id)?.peakStart"
            :peak-end="findOption(id)?.peakEnd"
            :peak-rate-multiplier="findOption(id)?.peakRateMultiplier"
          />
          <div class="flex shrink-0 items-center gap-0.5">
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition hover:bg-white hover:text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-primary-300"
              :disabled="disabled || index === 0"
              :aria-label="t('keys.routing.moveUp')"
              :title="t('keys.routing.moveUp')"
              @click="move(index, -1)"
            ><Icon name="chevronUp" size="sm" /></button>
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition hover:bg-white hover:text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-primary-300"
              :disabled="disabled || index === modelValue.length - 1"
              :aria-label="t('keys.routing.moveDown')"
              :title="t('keys.routing.moveDown')"
              @click="move(index, 1)"
            ><Icon name="chevronDown" size="sm" /></button>
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded-md text-gray-400 transition hover:bg-rose-50 hover:text-rose-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-rose-500 dark:hover:bg-rose-950/40 dark:hover:text-rose-300"
              :disabled="disabled"
              :aria-label="t('common.remove')"
              :title="t('common.remove')"
              @click="toggle(id)"
            ><Icon name="x" size="sm" /></button>
          </div>
        </li>
      </ol>
      <p v-else class="rounded-lg border border-dashed border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200">
        {{ t('keys.routing.emptySelection') }}
      </p>
    </section>

    <section aria-labelledby="available-groups-heading">
      <div class="mb-2 flex items-center justify-between gap-3">
        <h3 id="available-groups-heading" class="text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('keys.routing.availableGroups') }}
        </h3>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.routing.addHint') }}</span>
      </div>
      <div v-if="availableOptions.length" class="max-h-52 space-y-1 overflow-y-auto overscroll-contain rounded-lg border border-gray-200 p-1 dark:border-dark-600">
        <label
          v-for="option in availableOptions"
          :key="option.value"
          class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 transition hover:bg-gray-50 focus-within:bg-gray-50 dark:hover:bg-dark-700/70 dark:focus-within:bg-dark-700/70"
        >
          <input
            type="checkbox"
            class="h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700"
            :checked="false"
            :disabled="disabled || modelValue.length >= 16"
            @change="toggle(option.value)"
          />
          <span class="min-w-0 flex-1">
            <GroupBadge
              :name="option.label"
              :platform="option.platform"
              :subscription-type="option.subscriptionType"
              :rate-multiplier="option.rate"
              :user-rate-multiplier="option.userRate"
              :peak-rate-enabled="option.peakRateEnabled"
              :peak-start="option.peakStart"
              :peak-end="option.peakEnd"
              :peak-rate-multiplier="option.peakRateMultiplier"
            />
            <span v-if="option.description" class="mt-1 block line-clamp-1 pl-1 text-xs text-gray-500 dark:text-gray-400">{{ option.description }}</span>
          </span>
        </label>
      </div>
      <p v-else class="rounded-lg border border-dashed border-gray-200 px-3 py-3 text-center text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t(modelValue.length ? 'keys.routing.noMoreGroups' : 'common.noGroupsAvailable') }}
      </p>
    </section>

    <p class="rounded-lg bg-gray-50 px-3 py-2.5 text-xs leading-5 text-gray-600 dark:bg-dark-700/60 dark:text-gray-300">
      {{ t('keys.routing.billingHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'
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
const availableOptions = computed(() => props.options.filter(option => !props.modelValue.includes(option.value)))
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
