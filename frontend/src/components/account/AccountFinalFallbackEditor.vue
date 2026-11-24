<template>
  <section v-if="!authStore.isSimpleMode" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600">
    <div class="flex items-center justify-between gap-4">
      <span class="input-label mb-0" :title="t('admin.accounts.finalFallback.description')">{{ t('admin.accounts.finalFallback.title') }}</span>
      <Toggle :model-value="modelValue.enabled" :aria-label="t('admin.accounts.finalFallback.title')"
        @update:model-value="enabled => emit('update:modelValue', { ...modelValue, enabled })" />
    </div>
    <div v-if="modelValue.enabled" class="space-y-2">
      <GroupSelector :model-value="modelValue.groupIds" :groups="eligibleGroups" :platform="platform"
        :label="t('admin.accounts.finalFallback.groups')"
        @update:model-value="groupIds => emit('update:modelValue', { ...modelValue, groupIds })" />
      <p v-if="modelValue.groupIds.length === 0" class="text-xs text-amber-600">{{ t('admin.accounts.finalFallback.required') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import type { AccountPlatform, AccountType, Group } from '@/types'
import GroupSelector from '@/components/common/GroupSelector.vue'
import Toggle from '@/components/common/Toggle.vue'
import type { AccountFinalFallbackConfig } from './finalFallback'

const props = defineProps<{
  modelValue: AccountFinalFallbackConfig
  groups: Group[]
  platform?: AccountPlatform
  accountType?: AccountType
}>()
const emit = defineEmits<{ 'update:modelValue': [value: AccountFinalFallbackConfig] }>()
const { t } = useI18n()
const authStore = useAuthStore()
const eligibleGroups = computed(() => props.groups.filter(group =>
  group.status === 'active' && !(group.require_oauth_only && props.accountType === 'apikey')
))
</script>
