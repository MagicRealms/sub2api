<template>
  <div class="sw-segmented sw-chart-display" role="group" :aria-label="t('dashboard.barChart')">
    <button v-for="mode in modes" :key="mode.value" type="button" :class="{ 'is-active': modelValue === mode.value }" :aria-pressed="modelValue === mode.value" @click="$emit('update:modelValue', mode.value)"><WorkspaceIcon :name="mode.icon" class="h-4 w-4" />{{ t(mode.label) }}</button>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
defineProps<{ modelValue: 'line' | 'bar' }>()
defineEmits<{ 'update:modelValue': [value: 'line' | 'bar'] }>()
const { t } = useI18n()
const modes = [{ value: 'line', icon: 'pulse', label: 'dashboard.lineChart' }, { value: 'bar', icon: 'analysis', label: 'dashboard.barChart' }] as const
</script>

<style scoped>
.sw-chart-display button { display: inline-flex; align-items: center; justify-content: center; gap: 5px; white-space: nowrap; }
</style>
