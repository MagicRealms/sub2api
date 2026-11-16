<template>
  <article
    class="plaza-model-card card-hover group flex h-full min-w-0 cursor-pointer flex-col overflow-hidden p-4 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
    :style="{ '--plaza-accent': platformAccentColor(entry.platform) }"
    :data-model="entry.name"
    :aria-label="t('modelPlaza.cards.openModel', { name: entry.name })"
    role="button" tabindex="0" aria-haspopup="dialog"
    data-test="plaza-model-card"
    @click="emit('open', variant)"
    @keydown.enter.self.prevent="emit('open', variant)"
    @keydown.space.self.prevent="emit('open', variant)"
  >
    <div class="flex items-start gap-2.5">
      <span class="plaza-provider grid h-9 w-9 shrink-0 place-items-center rounded-xl ring-1 ring-inset ring-black/5 dark:ring-white/10">
        <PlatformIcon :platform="entry.platform as GroupPlatform" size="md" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-[10px] text-gray-500 dark:text-dark-400">
          <span>{{ modelPlazaPlatformLabel(entry.platform) }}</span>
          <span class="plaza-provider rounded px-1.5 py-0.5 font-medium">{{ modeLabel }}</span>
          <span data-test="model-rate" :title="t('modelPlaza.cards.effectiveRate')" class="font-semibold text-gray-700 dark:text-gray-200">{{ rate }}×</span>
        </div>
        <h2 class="mt-1 break-all text-sm font-semibold leading-5 text-gray-900 dark:text-gray-100">{{ entry.name }}</h2>
      </div>
      <button type="button" class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-primary-600 focus-visible:outline-primary-500 dark:hover:bg-dark-700 dark:hover:text-primary-400" :aria-label="t('modelPlaza.cards.copyModel', { name: entry.name })" @click.stop="copyToClipboard(entry.name)" @keydown.stop>
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
      </button>
    </div>

    <div class="my-3 flex min-w-0 items-center gap-2" @click.stop @keydown.stop>
      <label :for="selectId" class="shrink-0 text-[10px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.cards.availableGroups') }} <span class="tabular-nums">{{ entry.variants.length }}</span></label>
      <select :id="selectId" class="input min-w-0 flex-1 truncate rounded-lg py-1.5 pl-2 pr-6 text-[11px] dark:bg-dark-900 dark:text-gray-100" :value="variant.group.id" :aria-label="t('modelPlaza.cards.selectModelGroup', { name: entry.name })" data-test="model-group-select" @change="selectGroup">
        <option v-for="item in entry.variants" :key="item.group.id" :value="item.group.id">{{ item.group.name }} · {{ modelRate(item) }}×{{ item.group.is_exclusive ? ` · ${t('modelPlaza.badges.exclusive')}` : '' }}</option>
      </select>
    </div>

    <div class="plaza-price-panel flex flex-1 flex-col rounded-xl border px-3 py-2.5" aria-live="polite" aria-atomic="true">
      <p class="mb-2 flex items-center justify-between gap-1 text-[9px] text-gray-500 dark:text-dark-400"><span>{{ t('modelPlaza.cards.basePrice') }}</span><span v-if="tokenBilling">{{ t(currency === 'CNY' ? 'modelPlaza.table.unitPerMillionCny' : 'modelPlaza.table.unitPerMillion') }}</span></p>
      <dl v-if="tokenBilling" class="grid gap-2" :class="tokenPrices?.cache_read_price != null ? 'grid-cols-3' : 'grid-cols-2'">
        <div><dt class="text-[10px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.input') }}</dt><dd class="mt-1 break-all font-mono text-base font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-input-price">{{ tokenPrice(tokenPrices?.input_price) }}</dd></div>
        <div><dt class="text-[10px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.output') }}</dt><dd class="mt-1 break-all font-mono text-base font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-output-price">{{ tokenPrice(tokenPrices?.output_price) }}</dd></div>
        <div v-if="tokenPrices?.cache_read_price != null"><dt class="text-[10px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.cards.cachedInput') }}</dt><dd class="mt-1 break-all font-mono text-base font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-cache-price">{{ tokenPrice(tokenPrices.cache_read_price) }}</dd></div>
      </dl>
      <div v-else class="font-mono text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-test="model-request-price">{{ requestPrice }}<span class="ml-1 font-sans text-[10px] font-normal text-gray-500 dark:text-dark-400">{{ t(mode === BILLING_MODE_IMAGE ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') }}</span></div>
      <p v-if="!variant.model.pricing" class="mt-1 text-[10px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.detail.noPricing') }}</p>
    </div>
    <div class="mt-2 flex items-center gap-1.5 text-[9px] text-gray-400 dark:text-dark-500">
      <span class="sw-model-kind-badge">{{ t(`modelPlaza.kinds.${modelKind(variant.model)}`) }}</span>
      <span v-if="variant.group.is_exclusive" class="text-violet-500 dark:text-violet-300">{{ t('modelPlaza.badges.exclusive') }}</span>
      <span v-if="variant.group.subscription_type === 'subscription'">{{ t('modelPlaza.badges.subscription') }}</span>
      <span v-if="variablePricing" class="min-w-0 truncate">{{ t('modelPlaza.cards.variablePricing') }}</span>
      <Icon name="arrowRight" size="xs" class="ml-auto shrink-0 text-primary-400 transition-transform group-hover:translate-x-0.5" />
    </div>
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
import { platformAccentColor } from '@/utils/platformColors'
import type { GroupPlatform } from '@/types'
import { baseTokenPrices, modelKind, modelPlazaPlatformLabel, modelPricingCurrency, modelRate, type PlazaModelEntry, type PlazaModelVariant } from './modelCards'

const props = defineProps<{ entry: PlazaModelEntry }>()
const emit = defineEmits<{ open: [variant: PlazaModelVariant] }>()
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
const selectId = useId()
const selectedGroupId = ref<number>()
const variant = computed(() => props.entry.variants.find(v => v.group.id === selectedGroupId.value) ?? props.entry.variants[0]!)
const currency = computed(() => modelPricingCurrency(variant.value.model))
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
  const low = formatScaled(Math.min(...prices), 1, 2, currency.value)
  const high = formatScaled(Math.max(...prices), 1, 2, currency.value)
  return low === high ? low : `${low} – ${high}`
})

function tokenPrice(value: number | null | undefined): string {
  return value == null ? '-' : formatScaled(value * rate.value, 1_000_000, 2, currency.value)
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
