<template>
  <div class="sw-solar-scene" aria-hidden="true">
    <div class="sw-solar-halo"></div>
    <div class="sw-solar-orbit sw-solar-orbit-a"><i></i></div>
    <div class="sw-solar-orbit sw-solar-orbit-b"><i></i></div>
    <div class="sw-solar-rays"><span v-for="n in 12" :key="n" :style="{ '--ray': n }"></span></div>
    <div class="sw-solar-core"><span></span></div>
    <div class="sw-solar-satellite sw-solar-satellite-a"></div>
    <div class="sw-solar-satellite sw-solar-satellite-b"></div>
  </div>
</template>
