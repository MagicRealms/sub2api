<template>
  <div v-if="srcdoc" class="mt-4">
    <div
      class="relative w-full overflow-hidden rounded-md bg-white ring-1 ring-gray-200 dark:ring-dark-700"
      :style="{ aspectRatio: String(aspectRatio) }"
    >
      <iframe
        :srcdoc="srcdoc"
        class="absolute inset-0 h-full w-full border-0 bg-transparent"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
        loading="lazy"
        :title="title"
      ></iframe>
    </div>
  </div>
  <p v-else class="mt-4 rounded-md bg-amber-50 px-3 py-2.5 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
    {{ t('channelIQ.card.animationUnavailable') }}
  </p>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { extractChannelIQHtmlPreviewDocument, getChannelIQAnimationAspectRatio } from '@/utils/channelIQHtml'

const props = defineProps<{ responseText: string; title: string }>()
const { t } = useI18n()
const nonce = document.querySelector<HTMLScriptElement>('script[nonce]')?.nonce || ''
const srcdoc = computed(() => extractChannelIQHtmlPreviewDocument(props.responseText, nonce))
const aspectRatio = computed(() => getChannelIQAnimationAspectRatio(props.responseText))
</script>
