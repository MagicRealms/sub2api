<template>
  <!-- 后台内嵌形态:?embedded=1 且已登录,套完整后台布局 -->
  <AppLayout v-if="isEmbedded">
    <ModelPlazaContent :response="data" :loading="loading" :refreshing="refreshing" :error="loadFailed" :refresh-error="refreshFailed" embedded @refresh="loadModelPlaza" />
  </AppLayout>

  <!-- 独立形态:自带导航条(logo/站名 + 登录/回后台) -->
  <div v-else class="sunworld-shell min-h-screen">
    <PlazaNavBar />
    <main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
      <ModelPlazaContent :response="data" :loading="loading" :refreshing="refreshing" :error="loadFailed" :refresh-error="refreshFailed" @refresh="loadModelPlaza" />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlazaNavBar from '@/components/modelPlaza/PlazaNavBar.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

// embedded=1 但未登录(如转发的链接)自动降级为独立形态。
const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)

const data = ref<ModelPlazaResponse | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const loadFailed = ref(false)
const refreshFailed = ref(false)

const loadModelPlaza = async () => {
  const hasData = data.value !== null
  loading.value = !hasData
  refreshing.value = hasData
  loadFailed.value = false
  refreshFailed.value = false
  // The standalone navigation needs settings once; catalog refreshes do not.
  if (!hasData) void appStore.fetchPublicSettings()
  try {
    data.value = await getModelPlaza()
  } catch {
    if (hasData) refreshFailed.value = true
    else loadFailed.value = true
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

onMounted(() => {
  // 独立形态导航条需要站点名/Logo;有 __APP_CONFIG__ 注入时同步命中缓存。
  void loadModelPlaza()
})
</script>
