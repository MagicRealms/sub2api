<template>
  <div v-if="groups && groups.length > 0" class="relative max-w-56">
    <!-- 限制列宽但不裁切行高，完整显示分组名称和 +N 按钮 -->
    <div class="flex flex-wrap gap-1">
      <GroupBadge
        v-for="group in displayGroups"
        :key="group.id"
        :name="group.name"
        :platform="group.platform"
        :subscription-type="group.subscription_type"
        :rate-multiplier="group.rate_multiplier"
        :show-rate="false"
        :fallback-only="fallbackGroupIds.includes(group.id)"
        :truncate="false"
        class="min-w-0 max-w-full"
      />
      <!-- 更多数量徽章 -->
      <button
        v-if="hiddenCount > 0"
        ref="moreButtonRef"
        @click.stop="showPopover = !showPopover"
        :title="t('admin.accounts.finalFallback.membershipSummary', { normal: normalGroups.length, fallback: fallbackGroups.length })"
        class="inline-flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500 transition-colors cursor-pointer whitespace-nowrap"
      >
        <span>+{{ hiddenCount }}</span>
      </button>
    </div>

    <!-- Popover 显示完整列表 -->
    <FloatingPanel :open="showPopover" :anchor="moreButtonRef" :min-width="192" @close="showPopover = false" class="w-96 rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-dark-600 dark:bg-dark-800">
          <div class="mb-2 flex items-center justify-between">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.groupCountTotal', { count: groups.length }) }}
            </span>
            <button
              @click="showPopover = false"
              class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="max-h-64 space-y-3 overflow-y-auto">
            <div v-for="section in groupSections" :key="section.role" class="space-y-1.5">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ section.label }} · {{ section.groups.length }}</p>
              <div class="flex flex-wrap gap-1.5">
                <GroupBadge
                  v-for="group in section.groups"
                  :key="group.id"
                  :name="group.name"
                  :platform="group.platform"
                  :subscription-type="group.subscription_type"
                  :rate-multiplier="group.rate_multiplier"
                  :show-rate="false"
                  :fallback-only="section.role === 'fallback'"
                  :truncate="false"
                  class="min-w-0 max-w-full"
                />
              </div>
            </div>
          </div>
        </FloatingPanel>

    <!-- 点击外部关闭 popover -->

  </div>
  <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import FloatingPanel from '@/components/common/FloatingPanel.vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from '@/components/common/GroupBadge.vue'
import type { Group } from '@/types'

interface Props {
  groups: Group[] | null | undefined
  maxDisplay?: number
  fallbackGroupIds?: number[]
}

const props = withDefaults(defineProps<Props>(), {
  maxDisplay: 4,
  fallbackGroupIds: () => []
})

const { t } = useI18n()

const moreButtonRef = ref<HTMLElement | null>(null)
const showPopover = ref(false)
const normalGroups = computed(() => (props.groups ?? []).filter(group => !props.fallbackGroupIds.includes(group.id)))
const fallbackGroups = computed(() => (props.groups ?? []).filter(group => props.fallbackGroupIds.includes(group.id)))
const groupSections = computed(() => [
  { role: 'normal', label: t('admin.accounts.finalFallback.normalGroups'), groups: normalGroups.value },
  { role: 'fallback', label: t('admin.accounts.finalFallback.groups'), groups: fallbackGroups.value }
].filter(section => section.groups.length > 0))

// 显示的分组（最多显示 maxDisplay 个）
const displayGroups = computed(() => {
  if (!props.groups) return []
  if (props.groups.length <= props.maxDisplay) {
    return props.groups
  }
  // 留一个位置给 +N 按钮
  return props.groups.slice(0, props.maxDisplay - 1)
})

// 隐藏的数量
const hiddenCount = computed(() => {
  if (!props.groups) return 0
  if (props.groups.length <= props.maxDisplay) return 0
  return props.groups.length - (props.maxDisplay - 1)
})

</script>
