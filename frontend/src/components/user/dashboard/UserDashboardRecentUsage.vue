<template>
  <div class="sunworld-work-card card">
    <div class="sunworld-work-card-header">
      <div>
        <p class="sunworld-kicker">RECENT / ACTIVITY</p>
        <h2 class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.recentUsage') }}</h2>
      </div>
      <span class="badge badge-gray">{{ t('dashboard.last7Days') }}</span>
    </div>
    <div class="p-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
      <div v-else-if="data.length === 0" class="py-8">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <div v-else class="space-y-3">
        <div v-for="log in data" :key="log.id" class="sunworld-usage-row flex items-center justify-between rounded-xl p-3.5 transition-colors">
          <div class="flex items-center gap-3">
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-primary-100 dark:bg-primary-900/30">
              <WorkspaceIcon name="records" class="h-4 w-4 text-sky-500" />
            </div>
            <div>
              <p class="max-w-[18rem] truncate text-sm font-medium text-gray-900 dark:text-white" :title="log.model">{{ log.model }}</p>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(log.created_at) }}</p>
            </div>
          </div>
          <div class="text-right">
            <p class="text-sm font-semibold">
              <span class="text-green-600 dark:text-green-400" :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>

            </p>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ formatTokenMillions(log.input_tokens + log.output_tokens + (log.cache_read_tokens || 0) + (log.cache_creation_tokens || 0)) }}</p>
          </div>
        </div>

        <router-link to="/usage" class="flex items-center justify-center gap-2 py-3 text-sm font-medium text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300">
          {{ t('dashboard.viewAllUsage') }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
import { formatTokenMillions } from '@/utils/workspace'
import Icon from '@/components/icons/Icon.vue'
import { dashboardDateTime as formatDateTime } from '@/utils/dashboardTime'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>
