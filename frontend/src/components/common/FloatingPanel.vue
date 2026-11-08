<template>
  <span v-if="anchor === undefined" ref="anchorMarker" hidden aria-hidden="true"></span>
  <Teleport to="body">
    <div
      v-if="open"
      ref="panelRef"
      v-bind="$attrs"
      data-floating-panel
      :style="panelStyle"
      @click.stop
      @pointerdown.stop
    >
      <slot />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch, type CSSProperties } from 'vue'
import { floatingAnchorVisible, floatingLayer, floatingViewport, topFloatingSurface } from '@/utils/floating'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{
  open: boolean
  anchor?: HTMLElement | null
  align?: 'start' | 'end'
  minWidth?: number
  matchWidth?: boolean
  maxHeight?: number
}>(), { align: 'start', minWidth: 0, matchWidth: false, maxHeight: 480 })
const emit = defineEmits<{ close: [] }>()
const panelRef = ref<HTMLElement | null>(null)
// Inline search/action menus can anchor to their existing relative container.
// The hidden marker stays in that container while the panel is portaled to body.
const anchorMarker = ref<HTMLElement | null>(null)
const resolvedAnchor = computed(() => props.anchor === undefined ? anchorMarker.value?.parentElement ?? null : props.anchor)
const panelStyle = ref<CSSProperties>({ position: 'fixed', visibility: 'hidden' })
let frame = 0
let generation = 0
let observer: ResizeObserver | undefined

function updatePosition() {
  const anchor = resolvedAnchor.value
  const panel = panelRef.value
  if (!props.open || !anchor || !panel) return
  const rect = anchor.getBoundingClientRect()
  const padding = 8
  const viewport = floatingViewport(anchor)
  // A menu must disappear with its trigger, rather than floating over another page region.
  if (!floatingAnchorVisible(anchor)) {
    emit('close')
    return
  }
  const availableWidth = viewport.width
  const base: CSSProperties = {
    position: 'fixed', margin: '0', zIndex: floatingLayer(anchor),
    minWidth: `${Math.min(Math.max(props.minWidth, props.matchWidth ? rect.width : 0), availableWidth)}px`,
    maxWidth: `${availableWidth}px`, maxHeight: `${Math.min(props.maxHeight, viewport.height)}px`,
    overflowY: 'auto', overscrollBehavior: 'contain'
  }
  if (props.matchWidth) base.width = `${Math.min(Math.max(props.minWidth, rect.width), availableWidth)}px`
  // Apply bounds before measurement so narrow viewports and long content are measured correctly.
  Object.assign(panel.style, base)
  const size = panel.getBoundingClientRect()
  const below = Math.max(0, viewport.bottom - rect.bottom - padding)
  const above = Math.max(0, rect.top - viewport.top - padding)
  const placeAbove = below < size.height && above > below
  const room = placeAbove ? above : below
  const height = Math.min(size.height, props.maxHeight, room)
  const preferredLeft = props.align === 'end' ? rect.right - size.width : rect.left
  panelStyle.value = {
    ...base,
    left: `${Math.max(viewport.left, Math.min(preferredLeft, viewport.right - size.width))}px`,
    top: `${Math.max(viewport.top, Math.min(placeAbove ? rect.top - height - padding : rect.bottom + padding, viewport.bottom - height))}px`,
    maxHeight: `${Math.min(props.maxHeight, room)}px`, visibility: 'visible'
  }
}

function schedulePosition(event?: Event) {
  // Scrolling menu contents does not move its anchor.
  if (event?.target instanceof Node && panelRef.value?.contains(event.target)) return
  if (!frame) frame = window.requestAnimationFrame(() => { frame = 0; updatePosition() })
}

function handleOutside(event: Event) {
  const target = event.target
  if (target instanceof Node && !resolvedAnchor.value?.contains(target) && !panelRef.value?.contains(target)) emit('close')
}

function handleEscape(event: KeyboardEvent) {
  const active = document.activeElement
  if (event.defaultPrevented || event.key !== 'Escape' || !active || !panelRef.value || !topFloatingSurface(panelRef.value) || (!resolvedAnchor.value?.contains(active) && !panelRef.value.contains(active))) return
  event.preventDefault()
  event.stopPropagation()
  emit('close')
  const anchor = resolvedAnchor.value
  const focusTarget = anchor?.matches('button, input, [tabindex]') ? anchor : anchor?.querySelector<HTMLElement>('button, input, [tabindex]')
  focusTarget?.focus({ preventScroll: true })
}

function cleanup() {
  if (frame) window.cancelAnimationFrame(frame)
  frame = 0
  observer?.disconnect()
  observer = undefined
  window.removeEventListener('scroll', schedulePosition, true)
  window.removeEventListener('resize', schedulePosition)
  window.removeEventListener('sunworld:overlay-change', schedulePosition)
  window.visualViewport?.removeEventListener('resize', schedulePosition)
  window.visualViewport?.removeEventListener('scroll', schedulePosition)
  document.removeEventListener('pointerdown', handleOutside, true)
  window.removeEventListener('keydown', handleEscape, true)
}

watch(() => [props.open, resolvedAnchor.value] as const, async ([open]) => {
  const current = ++generation
  cleanup()
  if (!open) return
  panelStyle.value = { position: 'fixed', visibility: 'hidden' }
  await nextTick()
  if (current !== generation || !props.open) return
  updatePosition()
  window.addEventListener('scroll', schedulePosition, { capture: true, passive: true })
  window.addEventListener('resize', schedulePosition, { passive: true })
  window.addEventListener('sunworld:overlay-change', schedulePosition)
  window.visualViewport?.addEventListener('resize', schedulePosition, { passive: true })
  window.visualViewport?.addEventListener('scroll', schedulePosition, { passive: true })
  document.addEventListener('pointerdown', handleOutside, true)
  window.addEventListener('keydown', handleEscape, true)
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => schedulePosition())
    if (resolvedAnchor.value) observer.observe(resolvedAnchor.value)
    if (panelRef.value) observer.observe(panelRef.value)
  }
}, { flush: 'post', immediate: true })

onBeforeUnmount(() => { generation++; cleanup() })
</script>
