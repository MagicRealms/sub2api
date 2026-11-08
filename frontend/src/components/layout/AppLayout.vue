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
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="sunworld-main relative p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
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
