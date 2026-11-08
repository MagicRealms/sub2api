<template>
  <article class="sw-metric" :class="`sw-tone-${tone || 'mint'}`">
    <div class="sw-metric-top"><span>{{ label }}</span><WorkspaceIcon :name="icon" /></div>
    <div class="sw-metric-value"><slot name="value">{{ value }}</slot></div>
    <div class="sw-metric-detail"><slot>{{ detail }}</slot></div>
    <div v-if="bars" class="sw-metric-spark" aria-hidden="true"><i v-for="(bar, i) in bars" :key="i" :style="{ height: `${Math.max(4, bar)}%`, '--i': i }"></i></div>
  </article>
</template>
<script setup lang="ts">
import WorkspaceIcon from './WorkspaceIcon.vue'
defineProps<{ label: string; value?: string | number; detail?: string; icon?: string; tone?: string; bars?: number[] }>()
</script>
