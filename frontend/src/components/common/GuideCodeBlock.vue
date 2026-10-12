<template>
  <div class="min-w-0 overflow-hidden rounded-xl bg-slate-950 text-slate-200">
    <div class="flex items-center justify-between gap-3 border-b border-white/10 px-4 py-2.5">
      <span class="min-w-0 break-all font-mono text-xs text-slate-400">{{ label }}</span>
      <button type="button" class="flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-xs text-slate-300 transition hover:bg-white/10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-400" :aria-label="t('usageGuide.copyCode', { label })" @click="copyToClipboard(code)">
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
        {{ copied ? t('common.copied') : t('usageGuide.copy') }}
      </button>
    </div>
    <pre class="overflow-x-auto p-4 text-[13px] leading-7" tabindex="0"><code>{{ code }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

defineProps<{ label: string; code: string }>()
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
</script>
