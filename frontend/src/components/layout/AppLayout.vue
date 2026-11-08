<template>
  <div class="sunworld-shell min-h-screen">
    <!-- A quiet horizon grid keeps the workspace atmospheric without competing with data. -->
    <div class="sunworld-ambient" aria-hidden="true">
      <div class="sunworld-ambient-sun"></div>
      <div class="sunworld-ambient-grid"></div>
    </div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="sw-content relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[104px]' : 'lg:ml-60']"
      :inert="mobileNavOpen"
    >
      <!-- Header -->
      <AppHeader :heading="headerTitle" :subtitle="headerSubtitle" :icon="headerIcon" />

      <!-- Main Content -->
      <main class="sunworld-main relative p-4 md:p-6 lg:p-8" :class="{ 'sw-main-fill': fill }">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

defineProps<{ headerTitle?: string; headerSubtitle?: string; headerIcon?: string; fill?: boolean }>()

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isMobile = useMediaQuery('(max-width: 1023px)')
const mobileNavOpen = computed(() => isMobile.value && appStore.mobileOpen)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
.sw-main-fill { height: calc(100dvh - 5rem); width: 100%; max-width: none; display: flex; flex-direction: column; min-height: 0; padding: 16px; }
@media (max-width: 767px) { .sw-main-fill { height: calc(100dvh - 74px); } }
@media (min-width: 768px) { .sw-main-fill { padding: 20px 24px; } }
</style>
