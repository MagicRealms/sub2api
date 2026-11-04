<template>
  <div v-if="srcdoc" class="mt-4">
    <div ref="viewport" class="relative aspect-[8/5] w-full overflow-hidden rounded-md bg-white ring-1 ring-gray-200 dark:ring-dark-700">
      <iframe
        ref="frame"
        :srcdoc="srcdoc"
        :style="frameStyle"
        class="pointer-events-none absolute border-0"
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
const viewport = ref<HTMLElement | null>(null)
const playing = ref(props.autoplay)
const token = crypto.randomUUID()
const nonce = document.querySelector<HTMLScriptElement>('script[nonce]')?.nonce || ''
const srcdoc = computed(() => extractChannelIQHtmlPreviewDocument(props.responseText, nonce, token, props.autoplay))
const dimensions = ref({ width: 1200, height: 750 })
const viewportWidth = ref(600)
let resizeObserver: ResizeObserver | undefined

const frameStyle = computed(() => {
  const scale = viewportWidth.value / Math.max(dimensions.value.width, dimensions.value.height * 1.6)
  return {
    width: `${dimensions.value.width}px`,
    height: `${dimensions.value.height}px`,
    left: `${(viewportWidth.value - dimensions.value.width * scale) / 2}px`,
    top: `${(viewportWidth.value / 1.6 - dimensions.value.height * scale) / 2}px`,
    transform: `scale(${scale})`,
    transformOrigin: 'top left',
  }
})

function syncPlaying() {
  frame.value?.contentWindow?.postMessage({ kind: 'channel-iq-control', token, playing: playing.value }, '*')
}

function togglePlaying() {
  playing.value = !playing.value
  syncPlaying()
}

function onMessage(event: MessageEvent) {
  if (event.source !== frame.value?.contentWindow || event.data?.kind !== 'channel-iq-ready' || event.data.token !== token) return
  dimensions.value = {
    width: Math.max(1200, Math.min(2400, Number(event.data.width) || 1200)),
    height: Math.max(750, Math.min(1800, Number(event.data.height) || 750)),
  }
  syncPlaying()
}

watch(() => props.autoplay, value => {
  playing.value = value
  syncPlaying()
})

onMounted(() => {
  window.addEventListener('message', onMessage)
  resizeObserver = new ResizeObserver(entries => {
    viewportWidth.value = entries[0]?.contentRect.width || 600
  })
  if (viewport.value) resizeObserver.observe(viewport.value)
})

onUnmounted(() => {
  window.removeEventListener('message', onMessage)
  resizeObserver?.disconnect()
})
</script>
