<template>
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
import { nextTick, onBeforeUnmount, ref, watch, type CSSProperties } from 'vue'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{
  open: boolean
  anchor: HTMLElement | null
  align?: 'start' | 'end'
  minWidth?: number
  matchWidth?: boolean
  maxHeight?: number
}>(), { align: 'start', minWidth: 0, matchWidth: false, maxHeight: 480 })
const emit = defineEmits<{ close: [] }>()
const panelRef = ref<HTMLElement | null>(null)
const panelStyle = ref<CSSProperties>({ position: 'fixed', visibility: 'hidden' })
let frame = 0
let generation = 0
let observer: ResizeObserver | undefined

function updatePosition() {
  const anchor = props.anchor
  const panel = panelRef.value
  if (!props.open || !anchor || !panel) return
  const rect = anchor.getBoundingClientRect()
  const padding = 8
  const viewportWidth = window.innerWidth
  const viewportHeight = window.innerHeight
  const inDialog = anchor.closest('[role="dialog"], .modal-overlay')
  const header = !inDialog ? document.querySelector<HTMLElement>('.sunworld-header') : null
  const visibleTop = header && !header.contains(anchor) ? header.getBoundingClientRect().bottom : 0
  // A menu must disappear with its trigger, rather than floating over another page region.
  if (rect.width > 0 && (rect.bottom <= visibleTop || rect.top >= viewportHeight || rect.right <= 0 || rect.left >= viewportWidth)) {
    emit('close')
    return
  }
  for (let parent = anchor.parentElement; rect.width > 0 && parent && parent !== document.body; parent = parent.parentElement) {
    const overflow = getComputedStyle(parent)
    const bounds = parent.getBoundingClientRect()
    if ((/(auto|scroll|hidden|clip)/.test(overflow.overflowY) && (rect.bottom <= bounds.top || rect.top >= bounds.bottom)) ||
        (/(auto|scroll|hidden|clip)/.test(overflow.overflowX) && (rect.right <= bounds.left || rect.left >= bounds.right))) {
      emit('close')
      return
    }
  }
  const availableWidth = Math.max(0, viewportWidth - padding * 2)
  let layer = 40
  // Page menus stay below dialogs; menus inside a dialog follow that dialog's layer.
  if (anchor.closest('[role="dialog"], .modal-overlay, .driver-active-element')) {
    for (let parent: HTMLElement | null = anchor; parent && parent !== document.body; parent = parent.parentElement) {
      const zIndex = Number.parseInt(getComputedStyle(parent).zIndex, 10)
      if (Number.isFinite(zIndex)) layer = Math.max(layer, zIndex + 10)
    }
  }
  const base: CSSProperties = {
    position: 'fixed', margin: '0', zIndex: layer,
    minWidth: `${Math.min(Math.max(props.minWidth, props.matchWidth ? rect.width : 0), availableWidth)}px`,
    maxWidth: `${availableWidth}px`, maxHeight: `${Math.min(props.maxHeight, Math.max(0, viewportHeight - padding * 2))}px`,
    overflowY: 'auto', overscrollBehavior: 'contain'
  }
  if (props.matchWidth) base.width = `${Math.min(Math.max(props.minWidth, rect.width), availableWidth)}px`
  // Apply bounds before measurement so narrow viewports and long content are measured correctly.
  Object.assign(panel.style, base)
  const size = panel.getBoundingClientRect()
  const below = Math.max(0, viewportHeight - rect.bottom - padding * 2)
  const above = Math.max(0, rect.top - padding * 2)
  const placeAbove = below < size.height && above > below
  const room = placeAbove ? above : below
  const height = Math.min(size.height, props.maxHeight, room)
  const preferredLeft = props.align === 'end' ? rect.right - size.width : rect.left
  panelStyle.value = {
    ...base,
    left: `${Math.max(padding, Math.min(preferredLeft, viewportWidth - size.width - padding))}px`,
    top: `${Math.max(padding, Math.min(placeAbove ? rect.top - height - padding : rect.bottom + padding, viewportHeight - height - padding))}px`,
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
  if (target instanceof Node && !props.anchor?.contains(target) && !panelRef.value?.contains(target)) emit('close')
}

function handleEscape(event: KeyboardEvent) {
  const active = document.activeElement
  if (event.key !== 'Escape' || !active || (!props.anchor?.contains(active) && !panelRef.value?.contains(active))) return
  event.preventDefault()
  event.stopPropagation()
  emit('close')
  const anchor = props.anchor
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
  window.visualViewport?.removeEventListener('resize', schedulePosition)
  document.removeEventListener('pointerdown', handleOutside, true)
  window.removeEventListener('keydown', handleEscape, true)
}

watch(() => [props.open, props.anchor] as const, async ([open]) => {
  const current = ++generation
  cleanup()
  if (!open) return
  panelStyle.value = { position: 'fixed', visibility: 'hidden' }
  await nextTick()
  if (current !== generation || !props.open) return
  updatePosition()
  window.addEventListener('scroll', schedulePosition, { capture: true, passive: true })
  window.addEventListener('resize', schedulePosition, { passive: true })
  window.visualViewport?.addEventListener('resize', schedulePosition, { passive: true })
  document.addEventListener('pointerdown', handleOutside, true)
  window.addEventListener('keydown', handleEscape, true)
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => schedulePosition())
    if (props.anchor) observer.observe(props.anchor)
    if (panelRef.value) observer.observe(panelRef.value)
  }
}, { flush: 'post', immediate: true })

onBeforeUnmount(() => { generation++; cleanup() })
</script>
