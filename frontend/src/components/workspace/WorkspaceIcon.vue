<template>
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path :d="shape" />
    <circle v-if="accent" :cx="accent[0]" :cy="accent[1]" r="2.1" fill="currentColor" stroke="none" opacity=".3" />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ name?: string }>()
// An original, soft geometric icon family for the Sunworld console.
const shapes: Record<string, string> = {
  board: 'M4 13V7a3 3 0 0 1 3-3h10a3 3 0 0 1 3 3v10a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3m0-6h16M11 11v9M7 7.5h.01M14 16l1.5-2 1.5 1 2-2',
  key: 'M14 4a6 6 0 1 0 3.8 10.6L21 18v3h-4v-3h-3l-1.5-1.5M12 8h.01M15 7.5h.01',
  records: 'M8 4h10a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8m4-4v4H4l4-4Zm1 8h7m-7 4h4',
  analysis: 'M4 17V8m5 9V4m5 13v-6m5 6V7M3 21h18',
  models: 'm12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Zm0 0v9m8-4.5L12 12 4 7.5m8 4.5v9m-4-6.7 4 2.2 4-2.2',
  users: 'M15 8a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM3 21v-2a7 7 0 0 1 14 0v2M18 5a3 3 0 0 1 0 6m2 4a5 5 0 0 1 2 4',
  channels: 'M5 5h5v5H5V5Zm9 9h5v5h-5v-5ZM10 7.5h4a3.5 3.5 0 0 1 3.5 3.5v3M7.5 10v4a3.5 3.5 0 0 0 3.5 3.5h3',
  group: 'm12 3 9 5-9 5-9-5 9-5ZM3 12l9 5 9-5M3 16l9 5 9-5',
  pulse: 'M3 12h4l3-7 4 14 3-7h4',
  wallet: 'M20 7H6a2 2 0 0 1 0-4h11v4M4 5v13a3 3 0 0 0 3 3h13V7M16 12h5v5h-5a2.5 2.5 0 0 1 0-5Z',
  guide: 'M12 6C9 3 5 3 3 4v15c3-1 6-.5 9 2m0-15c3-3 7-3 9-2v15c-3-1-6-.5-9 2V6ZM7 8h1m8 0h1',
  settings: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8ZM12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2',
  bot: 'M12 3v3m-1-3h2M7 7h10a3 3 0 0 1 3 3v7a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3v-7a3 3 0 0 1 3-3ZM1 12v4m22-4v4M8 12v2m8-2v2m-6 3h4',
  lab: 'M9 3h6m-5 0v6L4 19a1.3 1.3 0 0 0 1 2h14a1.3 1.3 0 0 0 1-2L14 9V3M7 15h10m-8 3h.01m5-1h.01',
  image: 'M6 3h12a3 3 0 0 1 3 3v12a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3Zm-3 13 6-6 5 6 3-3 4 5M15 7h.01',
  sun: 'M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5',
}
const aliases: Record<string,string> = { dashboard:'board', chart:'analysis', document:'records', server:'channels', cube:'models', dollar:'wallet', clock:'pulse', folder:'group', user:'users', sparkles:'sun', beaker:'lab', book:'guide' }
const key = computed(() => aliases[props.name || ''] || props.name || 'board')
const shape = computed(() => shapes[key.value] || shapes.models)
const accent = computed(() => ({ board:[7,7], key:[14,8], records:[17,5], analysis:[19,7], models:[12,12], users:[11,8], channels:[7.5,7.5], wallet:[18,14.5] } as Record<string,number[]>)[key.value])
</script>
