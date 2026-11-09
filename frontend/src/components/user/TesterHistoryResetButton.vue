<template>
  <button v-if="auth.isTester" type="button" class="btn btn-secondary" :disabled="disabled || busy" @click="show = true">
    <Icon name="trash" size="sm" />{{ t('tester.clear') }}
  </button>
  <BaseDialog :show="show" :title="t('tester.clear')" width="narrow" :show-close-button="!busy" :close-on-escape="!busy" @close="close">
    <p class="text-sm leading-7 text-gray-600 dark:text-dark-300">{{ t('tester.clearHint') }}</p>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy" @click="clear">{{ busy ? t('tester.clearing') : t('tester.confirmClear') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { resetTesterHistory } from '@/api/tester'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

withDefaults(defineProps<{ disabled?: boolean }>(), { disabled: false })
const emit = defineEmits<{ reset: [visibleAfter: string] }>()
const auth = useAuthStore()
const app = useAppStore()
const { t } = useI18n()
const show = ref(false)
const busy = ref(false)
function close() { if (!busy.value) show.value = false }
async function clear() {
  if (busy.value || !auth.isTester) return
  busy.value = true
  try {
    const result = await resetTesterHistory()
    show.value = false
    app.showSuccess(t('tester.cleared'))
    emit('reset', result.visible_after)
  } catch { app.showError(t('tester.clearFailed')) }
  finally { busy.value = false }
}
</script>
