<template>
  <div class="sunworld-work-card card">
    <div class="sunworld-work-card-header">
      <div>
        <p class="sunworld-kicker">SHORTCUTS</p>
        <h2 class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.quickActions') }}</h2>
      </div>
      <Icon name="sparkles" size="sm" class="text-sun-500" />
    </div>
    <div class="space-y-2 p-3">
      <button @click="router.push('/keys')" class="sunworld-action-row group flex w-full items-center gap-4 rounded-xl p-3 text-left transition-all duration-200">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-primary-100 transition-transform group-hover:scale-105 dark:bg-primary-900/30">
          <WorkspaceIcon name="key" class="h-5 w-5 text-sky-500" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.createApiKey') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.generateNewKey') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-primary-500 dark:text-dark-500"
        />
      </button>

      <button @click="router.push('/usage')" class="sunworld-action-row group flex w-full items-center gap-4 rounded-xl p-3 text-left transition-all duration-200">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-100 transition-transform group-hover:scale-105 dark:bg-emerald-900/30">
          <WorkspaceIcon name="records" class="h-5 w-5 text-emerald-500" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.viewUsage') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.checkDetailedLogs') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-emerald-500 dark:text-dark-500"
        />
      </button>

      <button v-if="canUseBatchImage" @click="router.push('/batch-image')" class="sunworld-action-row group flex w-full items-center gap-4 rounded-xl p-3 text-left transition-all duration-200">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-sky-100 transition-transform group-hover:scale-105 dark:bg-sky-900/30">
          <WorkspaceIcon name="image" class="h-5 w-5 text-sky-500" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.batchImageAgent') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.batchImageAgentDesc') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-sky-500 dark:text-dark-500"
        />
      </button>

      <button @click="router.push('/redeem')" class="sunworld-action-row group flex w-full items-center gap-4 rounded-xl p-3 text-left transition-all duration-200">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-amber-100 transition-transform group-hover:scale-105 dark:bg-amber-900/30">
          <WorkspaceIcon name="wallet" class="h-5 w-5 text-amber-500" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.redeemCode') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.addBalanceWithCode') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-amber-500 dark:text-dark-500"
        />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
