<template>
  <div class="space-y-4" data-test="ordered-group-selector">
    <section aria-labelledby="selected-groups-heading">
      <div class="mb-2 flex items-center justify-between gap-3">
        <h3 id="selected-groups-heading" class="text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('keys.routing.selectedGroups') }}
        </h3>
        <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium tabular-nums text-gray-600 dark:bg-dark-700 dark:text-gray-300">
          {{ modelValue.length }}/16
        </span>
      </div>

      <ol v-if="modelValue.length" class="max-h-52 space-y-1.5 overflow-y-auto overscroll-contain" aria-live="polite">
        <li
          v-for="(id, index) in modelValue"
          :key="id"
          class="flex min-w-0 items-center gap-2 rounded-md border border-gray-200 bg-white px-2 py-1.5 dark:border-dark-600 dark:bg-dark-800"
          :data-route-id="id"
        >
          <span
            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-primary-600 text-xs font-semibold tabular-nums text-white"
            :aria-label="t('keys.routing.priority', { order: index + 1 })"
          >{{ String(index + 1).padStart(2, '0') }}</span>
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
              class="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition hover:bg-gray-100 hover:text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-primary-300"
              :disabled="disabled || index === 0"
              :aria-label="t('keys.routing.moveUp')"
              :title="t('keys.routing.moveUp')"
              @click="move(index, -1)"
            ><Icon name="chevronUp" size="sm" /></button>
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition hover:bg-gray-100 hover:text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-primary-300"
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
      <p v-else class="rounded-md border border-dashed border-gray-300 bg-gray-50 px-3 py-2.5 text-xs leading-5 text-gray-600 dark:border-dark-600 dark:bg-dark-800/60 dark:text-gray-300">
        {{ t('keys.routing.emptySelection') }}
      </p>
    </section>

    <section aria-labelledby="available-groups-heading">
      <div class="mb-2 flex items-center justify-between gap-3">
        <h3 id="available-groups-heading" class="text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('keys.routing.availableGroups') }}
        </h3>
        <span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ availableOptions.length }}</span>
      </div>

      <div v-if="availableOptions.length" class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-600">
        <label class="flex h-9 items-center gap-2 border-b border-gray-200 bg-gray-50 px-3 dark:border-dark-600 dark:bg-dark-800">
          <Icon name="search" size="sm" class="shrink-0 text-gray-400" />
          <input
            v-model="searchText"
            type="search"
            autocomplete="off"
            data-testid="available-group-search"
            :aria-label="t('keys.searchGroup')"
            :placeholder="t('keys.searchGroup')"
            class="min-w-0 flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400"
          />
          <button
            v-if="searchText"
            type="button"
            class="flex h-6 w-6 shrink-0 items-center justify-center rounded text-gray-400 hover:bg-gray-200 hover:text-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :aria-label="t('common.clear')"
            :title="t('common.clear')"
            @click="searchText = ''"
          ><Icon name="x" size="xs" /></button>
        </label>

        <div class="max-h-52 space-y-1 overflow-y-auto overscroll-contain p-1.5">
          <button
            v-for="option in filteredAvailableOptions"
            :key="option.value"
            type="button"
            data-group-option
            :data-group-id="option.value"
            class="group flex min-h-12 w-full min-w-0 items-center gap-2 rounded-md px-2.5 py-2 text-left transition hover:bg-gray-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-dark-700/70"
            :disabled="disabled || modelValue.length >= 16"
            :aria-label="`${t('keys.routing.addGroup')}: ${option.label}`"
            @click="toggle(option.value)"
          >
            <span class="min-w-0 flex-1">
              <GroupBadge
                class="min-w-0"
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
              <span v-if="option.description" class="mt-0.5 block truncate pl-1 text-xs text-gray-500 dark:text-gray-400">{{ option.description }}</span>
            </span>
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-gray-200 text-gray-500 transition group-hover:border-primary-300 group-hover:text-primary-700 dark:border-dark-600 dark:text-gray-400 dark:group-hover:text-primary-300">
              <Icon name="plus" size="sm" />
            </span>
          </button>

          <p v-if="filteredAvailableOptions.length === 0" class="px-3 py-4 text-center text-xs text-gray-500 dark:text-gray-400">
            {{ t('keys.noGroupFound') }}
          </p>
        </div>
      </div>
      <p v-else class="rounded-md border border-dashed border-gray-200 px-3 py-3 text-center text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t(modelValue.length ? 'keys.routing.noMoreGroups' : 'common.noGroupsAvailable') }}
      </p>
    </section>

    <p class="rounded-md bg-gray-50 px-3 py-2.5 text-xs leading-5 text-gray-600 dark:bg-dark-700/60 dark:text-gray-300">
      {{ t('keys.routing.billingHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
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

const props = defineProps<{
  modelValue: number[]
  options: OrderedGroupOption[]
  allOptions?: OrderedGroupOption[]
  disabled?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const { t } = useI18n()
const searchText = ref('')
const findOption = (id: number) => (props.allOptions ?? props.options).find(option => option.value === id)
const availableOptions = computed(() => props.options.filter(option => !props.modelValue.includes(option.value)))
const filteredAvailableOptions = computed(() => {
  const query = searchText.value.trim().toLocaleLowerCase()
  if (!query) return availableOptions.value
  return availableOptions.value.filter(option =>
    option.label.toLocaleLowerCase().includes(query) ||
    option.description?.toLocaleLowerCase().includes(query)
  )
})

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
