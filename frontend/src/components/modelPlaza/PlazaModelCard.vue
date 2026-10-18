<template>
  <article
    class="plaza-model-card flex h-full min-w-0 flex-col overflow-hidden rounded-2xl border border-gray-200/80 bg-white p-5 shadow-card transition-shadow hover:shadow-card-hover dark:border-dark-700/70 dark:bg-dark-800/70"
    :style="{ '--plaza-accent': platformAccentColor(entry.platform) }"
    :data-model="entry.name"
    data-test="plaza-model-card"
  >
    <div class="flex items-start gap-3">
      <span class="plaza-provider grid h-10 w-10 shrink-0 place-items-center rounded-xl ring-1 ring-inset ring-black/5 dark:ring-white/10">
        <PlatformIcon :platform="entry.platform as GroupPlatform" size="md" />
      </span>
      <div class="min-w-0 flex-1">
        <p class="text-[11px] text-gray-500 dark:text-dark-400">{{ platformLabel(entry.platform) }}</p>
        <h2 class="mt-0.5 break-all font-mono text-sm font-semibold leading-6 text-gray-900 dark:text-gray-100">{{ entry.name }}</h2>
      </div>
      <button type="button" class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-primary-600 focus-visible:outline-primary-500 dark:hover:bg-dark-700 dark:hover:text-primary-400" :aria-label="t('modelPlaza.cards.copyModel', { name: entry.name })" @click="copyToClipboard(entry.name)">
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
      </button>
    </div>

    <div class="mt-5">
      <label :for="selectId" class="mb-2 flex items-center justify-between gap-2 text-[11px] text-gray-500 dark:text-dark-400">
        <span>{{ t('modelPlaza.cards.availableGroups') }}</span>
        <span>{{ entry.variants.length }}</span>
      </label>
      <select :id="selectId" class="input w-full min-w-0 truncate rounded-xl py-2.5 pl-3 pr-7 text-xs dark:bg-dark-900 dark:text-gray-100" :value="variant.group.id" :aria-label="t('modelPlaza.cards.selectModelGroup', { name: entry.name })" data-test="model-group-select" @change="selectGroup">
        <option v-for="item in entry.variants" :key="item.group.id" :value="item.group.id">{{ item.group.name }} · {{ modelRate(item) }}×{{ item.group.is_exclusive ? ` · ${t('modelPlaza.badges.exclusive')}` : '' }}</option>
      </select>
    </div>

    <div class="my-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px]">
      <span class="plaza-provider rounded-md px-2 py-1 font-medium">{{ modeLabel }}</span>
      <span v-if="variant.group.is_exclusive" class="text-violet-600 dark:text-violet-300">{{ t('modelPlaza.badges.exclusive') }}</span>
      <span v-if="variant.group.subscription_type === 'subscription'" class="text-gray-500 dark:text-dark-400">{{ t('modelPlaza.badges.subscription') }}</span>
      <span class="ml-auto text-gray-500 dark:text-dark-400" data-test="model-rate">{{ t('modelPlaza.cards.effectiveRate') }} <strong class="font-semibold text-gray-700 dark:text-gray-200">{{ rate }}×</strong></span>
    </div>

    <div class="flex flex-1 flex-col rounded-xl bg-gray-50 p-3.5 dark:bg-dark-900/50" aria-live="polite" aria-atomic="true">
      <p class="mb-3 text-[10px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.cards.basePrice') }}<span v-if="tokenBilling" class="float-right">{{ t('modelPlaza.table.unitPerMillion') }}</span></p>
      <dl v-if="tokenBilling" class="grid grid-cols-2 gap-3">
        <div><dt class="text-[11px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.input') }}</dt><dd class="mt-1 break-all font-mono text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-input-price">{{ tokenPrice(tokenPrices?.input_price) }}</dd></div>
        <div><dt class="text-[11px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.output') }}</dt><dd class="mt-1 break-all font-mono text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-output-price">{{ tokenPrice(tokenPrices?.output_price) }}</dd></div>
      </dl>
      <div v-else class="font-mono text-xl font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-request-price">{{ requestPrice }}<span class="ml-1 font-sans text-xs font-normal text-gray-500 dark:text-dark-400">{{ t(mode === BILLING_MODE_IMAGE ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') }}</span></div>
      <div v-if="tokenBilling && tokenPrices?.cache_read_price != null" class="mt-3 flex justify-between gap-2 border-t border-gray-200/60 pt-2.5 text-[11px] dark:border-dark-700/70"><span class="text-gray-500 dark:text-dark-400">{{ t('modelPlaza.cards.cachedInput') }}</span><span class="font-mono text-gray-700 dark:text-dark-300" data-test="model-cache-price">{{ tokenPrice(tokenPrices.cache_read_price) }}</span></div>
      <p v-if="!variant.model.pricing" class="mt-2 text-[11px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.detail.noPricing') }}</p>
    </div>

    <p class="mt-3 min-h-8 text-[10px] leading-4 text-gray-400 dark:text-dark-500">{{ variablePricing ? t('modelPlaza.cards.variablePricing') : t('modelPlaza.cards.switchHint') }}</p>
    <button type="button" class="mt-2 flex w-full items-center justify-between rounded-lg py-2 text-xs font-semibold text-primary-600 transition hover:text-primary-700 focus-visible:outline-primary-500 dark:text-primary-400 dark:hover:text-primary-300" :aria-label="t('modelPlaza.cards.openModel', { name: entry.name })" data-test="model-pricing-button" @click="emit('open', variant)">
      {{ t('modelPlaza.cards.viewPricing') }}<Icon name="arrowRight" size="sm" />
    </button>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { BILLING_MODE_IMAGE, BILLING_MODE_TOKEN } from '@/constants/channel'
import { formatScaled } from '@/utils/pricing'
import { platformAccentColor, platformLabel } from '@/utils/platformColors'
import type { GroupPlatform } from '@/types'
import { baseTokenPrices, modelRate, type PlazaModelEntry, type PlazaModelVariant } from './modelCards'

const props = defineProps<{ entry: PlazaModelEntry }>()
const emit = defineEmits<{ open: [variant: PlazaModelVariant] }>()
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
const selectId = useId()
const selectedGroupId = ref<number>()
const variant = computed(() => props.entry.variants.find(v => v.group.id === selectedGroupId.value) ?? props.entry.variants[0]!)
const rate = computed(() => modelRate(variant.value))
const mode = computed(() => variant.value.model.pricing?.billing_mode ?? BILLING_MODE_TOKEN)
const tokenBilling = computed(() => mode.value === BILLING_MODE_TOKEN)
const modeLabel = computed(() => tokenBilling.value ? t('modelPlaza.cards.perToken') : t(mode.value === BILLING_MODE_IMAGE ? 'modelPlaza.table.perImage' : 'modelPlaza.table.perRequest'))
const tokenPrices = computed(() => baseTokenPrices(variant.value.model))
const variablePricing = computed(() => (variant.value.model.pricing?.intervals.length ?? 0) > 1 || !!variant.value.model.time_pricing?.periods.length || variant.value.group.peak_rate_enabled || Object.keys(variant.value.model.pricing?.reasoning_effort_multipliers ?? {}).length > 0)
const requestPrice = computed(() => {
  const pricing = variant.value.model.pricing
  const values = pricing?.intervals.flatMap(iv => iv.per_request_price == null ? [] : [iv.per_request_price]) ?? []
  if (!values.length && pricing?.per_request_price != null) values.push(pricing.per_request_price)
  if (!values.length) return '-'
  const prices = values.map(value => value * rate.value)
  const low = formatScaled(Math.min(...prices), 1, 2)
  const high = formatScaled(Math.max(...prices), 1, 2)
  return low === high ? low : `${low} – ${high}`
})

function tokenPrice(value: number | null | undefined): string {
  return value == null ? '-' : formatScaled(value * rate.value, 1_000_000, 2)
}

function selectGroup(event: Event) {
  selectedGroupId.value = Number((event.target as HTMLSelectElement).value)
}
</script>

<style scoped>
.plaza-provider {
  color: var(--plaza-accent);
  background: color-mix(in srgb, var(--plaza-accent) 9%, transparent);
}
:global(.dark) .plaza-provider {
  color: color-mix(in srgb, var(--plaza-accent) 65%, white);
  background: color-mix(in srgb, var(--plaza-accent) 12%, transparent);
}
</style>
