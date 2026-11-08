<template>
  <svg viewBox="0 0 100 100" fill="none" aria-hidden="true" class="studio-bot" :class="{ 'studio-bot-busy': active }">
    <ellipse cx="50" cy="88" rx="23" ry="4" fill="currentColor" opacity=".09" />
    <g class="bot-body">
      <path d="M50 24v-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
      <circle class="bot-antenna" cx="50" cy="12" r="4" fill="#edbc70" />
      <rect x="16" y="40" width="8" height="20" rx="4" fill="currentColor" opacity=".35" />
      <rect x="76" y="40" width="8" height="20" rx="4" fill="currentColor" opacity=".35" />
      <rect x="23" y="25" width="54" height="50" rx="19" fill="currentColor" opacity=".12" />
      <rect x="23" y="25" width="54" height="50" rx="19" stroke="currentColor" stroke-width="2.3" />
      <rect x="31" y="36" width="38" height="27" rx="10" fill="currentColor" opacity=".1" />
      <g class="bot-eyes" fill="currentColor"><rect x="39" y="44" width="5" height="8" rx="2.5" /><rect x="56" y="44" width="5" height="8" rx="2.5" /></g>
      <path d="M45 57q5 4 10 0" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
      <path d="M39 76v5m22-5v5" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
      <path d="m81 22 2-5 2 5 5 2-5 2-2 5-2-5-5-2 5-2Z" fill="#edbc70" opacity=".8" />
    </g>
  </svg>
</template>
<script setup lang="ts">
withDefaults(defineProps<{ active?: boolean }>(), { active: false })
</script>
<style scoped>
.studio-bot { overflow: visible; }
.bot-body { animation: bot-float 5s ease-in-out infinite; transform-origin: 50px 50px; }
.bot-eyes { animation: bot-blink 6s infinite; transform-origin: 50px 48px; }
.studio-bot-busy .bot-antenna { animation: bot-signal 1.2s ease-in-out infinite; }
@keyframes bot-float { 0%,100% { transform: translateY(0) rotate(-2deg); } 50% { transform: translateY(-4px) rotate(2deg); } }
@keyframes bot-blink { 0%,43%,47%,100% { transform: scaleY(1); } 45% { transform: scaleY(.15); } }
@keyframes bot-signal { 50% { opacity: .35; } }
@media (prefers-reduced-motion: reduce) { .bot-body,.bot-eyes,.studio-bot-busy .bot-antenna { animation: none; } }
</style>
