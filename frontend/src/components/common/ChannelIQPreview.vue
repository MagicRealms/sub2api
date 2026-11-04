<template>
  <div v-if="srcdoc" class="mt-4">
    <div class="relative aspect-[8/5] w-full overflow-hidden rounded-md bg-white ring-1 ring-gray-200 dark:ring-dark-700">
      <iframe
        ref="frame"
        :srcdoc="srcdoc"
        class="pointer-events-none absolute inset-0 h-full w-full border-0 bg-transparent"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
        :loading="autoplay ? 'eager' : 'lazy'"
        :title="title"
        @load="syncPlaying"
      ></iframe>
    </div>
    <div class="mt-2 flex justify-end">
      <button
        type="button"
        class="inline-flex h-8 w-8 items-center justify-center rounded border border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700"
        :title="playing ? t('channelIQ.card.pause') : t('channelIQ.card.play')"
        :aria-label="playing ? t('channelIQ.card.pause') : t('channelIQ.card.play')"
        @click="togglePlaying"
      >
        <Icon :name="playing ? 'pause' : 'play'" size="sm" />
      </button>
    </div>
  </div>
  <p v-else class="mt-4 rounded-md bg-amber-50 px-3 py-2.5 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
    {{ t('channelIQ.card.animationUnavailable') }}
  </p>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { extractChannelIQHtmlPreviewDocument } from '@/utils/channelIQHtml'

const props = defineProps<{ responseText: string; title: string; autoplay: boolean }>()
const { t } = useI18n()
const frame = ref<HTMLIFrameElement | null>(null)
const playing = ref(props.autoplay)
const token = crypto.randomUUID()
const nonce = document.querySelector<HTMLScriptElement>('script[nonce]')?.nonce || ''
const srcdoc = computed(() => extractChannelIQHtmlPreviewDocument(props.responseText, nonce, token, props.autoplay))

function syncPlaying() {
  frame.value?.contentWindow?.postMessage({ kind: 'channel-iq-control', token, playing: playing.value }, '*')
}

function togglePlaying() {
  playing.value = !playing.value
  syncPlaying()
}

function onMessage(event: MessageEvent) {
  if (event.source !== frame.value?.contentWindow || event.data?.kind !== 'channel-iq-ready' || event.data.token !== token) return
  syncPlaying()
}

watch(() => props.autoplay, value => {
  playing.value = value
  syncPlaying()
})

onMounted(() => {
  window.addEventListener('message', onMessage)
})

onUnmounted(() => {
  window.removeEventListener('message', onMessage)
})
</script>
