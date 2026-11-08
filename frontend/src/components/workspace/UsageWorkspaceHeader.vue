<template>
  <div class="sw-page-heading">
    <div>
      <div class="sw-page-kicker">{{ admin ? 'OPERATIONS / INSIGHTS' : 'YOUR / INSIGHTS' }}</div>
      <h1>{{ t(admin ? 'workspace.globalRecords' : 'usage.title') }}</h1>
      <p>{{ t(admin ? 'workspace.globalRecordsDescription' : 'workspace.recordsDescription') }}</p>
    </div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ admin?: boolean }>()
const { t } = useI18n()
</script>
