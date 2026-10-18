<template>
  <button
    type="button"
    class="plaza-group-card group flex h-full w-full min-w-0 flex-col rounded-2xl border border-gray-200/80 bg-white/70 p-5 text-left shadow-card backdrop-blur-xl transition-all duration-300 hover:-translate-y-1 hover:border-gray-300 hover:shadow-card-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:border-dark-700/70 dark:bg-dark-800/60 dark:hover:border-primary-500/30 dark:focus-visible:ring-offset-dark-900 motion-reduce:transform-none motion-reduce:transition-none"
    :style="{ '--plaza-accent': platformAccentColor(group.platform) }"
    :aria-label="t('modelPlaza.cards.openGroup', { name: group.name })"
    data-test="plaza-group-card"
    @click="emit('open', group.id)"
  >
    <div class="flex w-full items-start gap-3">
      <span class="plaza-provider grid h-10 w-10 shrink-0 place-items-center rounded-xl ring-1 ring-inset ring-black/5 dark:ring-white/10">
        <PlatformIcon :platform="group.platform as GroupPlatform" size="md" />
      </span>
      <div class="min-w-0 flex-1">
        <h2 class="break-words text-base font-semibold leading-6 text-gray-900 dark:text-gray-100">{{ group.name }}</h2>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ platformLabel(group.platform) }}</p>
      </div>
      <span
        class="shrink-0 rounded-full px-2.5 py-1 text-[11px] font-medium"
        :class="group.is_exclusive ? 'bg-violet-50 text-violet-600 dark:bg-violet-500/10 dark:text-violet-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700/70 dark:text-dark-300'"
      >{{ group.is_exclusive ? t('modelPlaza.badges.exclusive') : t('modelPlaza.cards.publicGroup') }}</span>
    </div>

    <p class="mt-3 line-clamp-2 min-h-10 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ group.description || t('modelPlaza.cards.groupHint') }}</p>

    <dl class="my-4 grid w-full grid-cols-2 gap-2.5">
      <div class="rounded-xl bg-gray-50 px-3.5 py-3 dark:bg-dark-900/40">
        <dt class="text-[11px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.cards.effectiveRate') }}</dt>
        <dd class="mt-1 text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">{{ rate }}<span class="ml-1 text-sm font-normal text-gray-400">×</span></dd>
      </div>
      <div class="rounded-xl bg-gray-50 px-3.5 py-3 dark:bg-dark-900/40">
        <dt class="text-[11px] text-gray-500 dark:text-dark-400">{{ t('modelPlaza.cards.modelCount') }}</dt>
        <dd class="mt-1 text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">{{ group.models.length }}<span class="ml-1.5 text-xs font-normal text-gray-400">{{ t('modelPlaza.cards.modelsUnit') }}</span></dd>
      </div>
    </dl>

    <div class="w-full flex-1 border-t border-gray-100 pt-4 dark:border-dark-700/60">
      <div class="mb-2 flex items-center justify-between text-[11px] text-gray-400 dark:text-dark-500">
        <span>{{ t('modelPlaza.cards.modelPreview') }}</span>
        <span v-if="group.subscription_type === 'subscription'">{{ t('modelPlaza.badges.subscription') }}</span>
      </div>
      <div class="flex min-h-[60px] flex-wrap content-start gap-1.5">
        <span v-for="model in previewModels" :key="`${model.platform}:${model.name}`" class="max-w-full truncate rounded-md border border-gray-100 bg-white/50 px-2 py-1 font-mono text-[11px] text-gray-600 dark:border-dark-700/70 dark:bg-dark-800/50 dark:text-dark-300" :title="model.name">{{ model.name }}</span>
        <span v-if="remaining > 0" class="plaza-provider rounded-md px-2 py-1 text-[11px] font-medium">+{{ remaining }}</span>
      </div>
    </div>

    <div class="mt-5 flex w-full items-center justify-between border-t border-gray-100 pt-3.5 dark:border-dark-700/60">
      <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('modelPlaza.cards.pricingHint') }}</span>
      <span class="inline-flex items-center gap-1 text-xs font-semibold text-primary-600 dark:text-primary-400">{{ t('modelPlaza.cards.viewPricing') }}<Icon name="arrowRight" size="sm" class="transition-transform group-hover:translate-x-0.5 motion-reduce:transform-none" /></span>
    </div>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformAccentColor, platformLabel } from '@/utils/platformColors'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type { GroupPlatform } from '@/types'

const props = defineProps<{ group: ModelPlazaGroup }>()
const emit = defineEmits<{ open: [id: number] }>()
const { t } = useI18n()
const rate = computed(() => props.group.user_rate_multiplier ?? props.group.rate_multiplier)
const previewModels = computed(() => {
  const sorted = [...props.group.models].sort((a, b) => b.name.localeCompare(a.name, undefined, { numeric: true }))
  const images = sorted.filter(m => /image|dall-e/i.test(m.name))
  const other = sorted.filter(m => !images.includes(m))
  return images.length && other.length ? [...other.slice(0, 4), ...images.slice(0, 2)] : sorted.slice(0, 6)
})
const remaining = computed(() => Math.max(0, props.group.models.length - previewModels.value.length))
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
