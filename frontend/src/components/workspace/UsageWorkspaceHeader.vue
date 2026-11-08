<template>
  <div class="sw-page-heading">
    <div>
      <div class="sw-page-kicker">{{ admin ? 'OPERATIONS / INSIGHTS' : 'YOUR / INSIGHTS' }}</div>
      <h1>{{ t(analysis ? (admin ? 'workspace.globalAnalytics' : 'workspace.analytics') : 'usage.title') }}</h1>
      <p>{{ t(analysis ? 'workspace.analyticsDescription' : 'workspace.recordsDescription') }}</p>
    </div>
  </div>
  <nav class="sw-usage-nav" :aria-label="t('workspace.usageNavigation')">
    <div class="sw-segmented">
      <router-link :to="base" :class="{ 'is-active': !analysis }" :aria-current="!analysis ? 'page' : undefined"><WorkspaceIcon name="records" />{{ t('usage.title') }}</router-link>
      <router-link :to="`${base}/analytics`" :class="{ 'is-active': analysis }" :aria-current="analysis ? 'page' : undefined"><WorkspaceIcon name="analysis" />{{ t('workspace.analytics') }}</router-link>
    </div>
    <span class="sw-section-meta">{{ t(admin ? 'workspace.globalScope' : 'workspace.personalScope') }}</span>
  </nav>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceIcon from './WorkspaceIcon.vue'
const props = defineProps<{ analysis?: boolean; admin?: boolean }>()
const base = computed(() => props.admin ? '/admin/usage' : '/usage')
const { t } = useI18n()
</script>
