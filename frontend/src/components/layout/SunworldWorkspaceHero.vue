<template>
  <section class="sunworld-hero">
    <div class="sunworld-hero-copy">
      <div class="sunworld-hero-eyebrow">
        <span class="sunworld-hero-mark"><Icon name="sun" size="sm" /></span>
        <span>{{ eyebrow }}</span>
      </div>
      <h2 class="sunworld-hero-title">{{ title }}</h2>
      <p class="sunworld-hero-subtitle">{{ subtitle }}</p>
      <router-link v-if="actionPath && actionLabel" :to="actionPath" class="sunworld-hero-action">
        <span>{{ actionLabel }}</span>
        <Icon name="arrowRight" size="sm" />
      </router-link>
    </div>

    <div class="sunworld-hero-metrics">
      <SolarScene />
      <div class="sunworld-hero-status">
        <span class="sunworld-hero-status-dot"></span>
        <div>
          <p class="sunworld-hero-status-label">{{ statusLabel }}</p>
          <p class="sunworld-hero-status-detail">{{ statusDetail }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import SolarScene from '@/components/workspace/SolarScene.vue'

defineProps<{
  eyebrow: string
  title: string
  subtitle: string
  actionLabel?: string
  actionPath?: string
  statusLabel: string
  statusDetail: string
}>()
</script>
