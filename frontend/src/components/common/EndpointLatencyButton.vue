<template>
  <span class="sw-endpoint-probe">
    <output v-if="latency !== null || failed" aria-live="polite" :class="{ 'is-failed': failed }">{{ failed ? t('common.error') : `${latency} ms` }}</output>
    <button type="button" class="btn btn-secondary" :disabled="busy" :aria-label="t('workspace.testConnection')" :title="t('workspace.connectionHint')" @click="probe"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" :class="{ 'animate-pulse': busy }" aria-hidden="true"><path d="m13 2-9 12h7l-1 8 10-13h-7l1-7Z" /></svg></button>
  </span>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { measureEndpointLatency } from '@/utils/endpointLatency'
const props = defineProps<{ endpoint: string }>()
const { t } = useI18n()
const latency = ref<number | null>(null), busy = ref(false), failed = ref(false)
let controller: AbortController | undefined
let timeout: ReturnType<typeof setTimeout> | undefined
const cancel = () => { controller?.abort(); controller = undefined; clearTimeout(timeout) }
watch(() => props.endpoint, () => { cancel(); latency.value = null; failed.value = false; busy.value = false })
onBeforeUnmount(cancel)
async function probe() {
  cancel()
  const current = new AbortController()
  controller = current
  busy.value = true; failed.value = false; latency.value = null
  timeout = setTimeout(() => current.abort(), 8000)
  try { const value = await measureEndpointLatency(props.endpoint, current.signal); if (controller === current) latency.value = value }
  catch { if (controller === current) failed.value = true }
  finally { if (controller === current) { clearTimeout(timeout); busy.value = false } }
}
</script>
<style scoped>
.sw-endpoint-probe { display: inline-flex; align-items: center; gap: 7px; flex-shrink: 0; }
output { font-size: 10px; font-variant-numeric: tabular-nums; color: var(--sw-mint); white-space: nowrap; }
output.is-failed { color: #ce7a65; }
svg { width: 16px; height: 16px; }
</style>
