<script setup lang="ts">
import { onBeforeUnmount, ref, useTemplateRef, nextTick, useId, watch, type CSSProperties } from 'vue'
import { floatingAnchorVisible, floatingLayer, floatingViewport, topFloatingSurface } from '@/utils/floating'

const props = withDefaults(defineProps<{
  content?: string
  trigger?: 'hover' | 'click'
  widthClass?: string
  focusable?: boolean
  disabled?: boolean
  touchTrigger?: boolean
  panelClass?: string
  variant?: 'dark' | 'surface'
}>(), {
  trigger: 'hover',
  widthClass: 'w-64',
  focusable: true,
  disabled: false,
  touchTrigger: true,
  panelClass: '',
  variant: 'dark',
})

const show = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('trigger')
const tooltipRef = useTemplateRef<HTMLElement>('tooltip')
const tooltipStyle = ref<CSSProperties>({ top: '0px', left: '0px' })
const tooltipBelow = ref(false)
const tooltipId = `help-${useId()}`
let pointerType = ''
let touchWasOpen = false
let frame = 0
let observer: ResizeObserver | undefined
let generation = 0
let destroyed = false

function openTooltip() {
  if (props.disabled) return
  show.value = true
}

function closeTooltip() {
  if (tooltipRef.value?.contains(document.activeElement)) {
    const trigger = triggerRef.value
    const target = props.focusable ? trigger : trigger?.querySelector<HTMLElement>('button, a[href], [tabindex]')
    target?.focus({ preventScroll: true })
  }
  show.value = false
}

function onEnter() {
  if (props.trigger !== 'hover' || pointerType === 'touch') return
  openTooltip()
}

function onPointerDown(event: PointerEvent) {
  pointerType = event.pointerType
  if (pointerType === 'touch') touchWasOpen = show.value
}

function isInside(container: HTMLElement | null, target: EventTarget | null): boolean {
  return target instanceof Node && !!container?.contains(target)
}

// 悬停模式下指针在触发图标与提示框之间往返时保持打开，便于选中提示里的文字。
function onLeave(event: MouseEvent) {
  if (props.trigger !== 'hover' || pointerType === 'touch') return
  if (isInside(tooltipRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onTooltipLeave(event: MouseEvent) {
  if (props.trigger !== 'hover' || pointerType === 'touch') return
  if (isInside(triggerRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onClick(event: MouseEvent) {
  if (props.disabled || (props.trigger === 'hover' && !props.touchTrigger)) return
  if (props.trigger !== 'click' && pointerType !== 'touch' && !window.matchMedia('(hover: none)').matches) return
  // Custom triggers may contain their own links or copy buttons.
  const action = (event.target as Element)?.closest('a, button, [role="button"]')
  if (action && action !== triggerRef.value) return
  event.stopPropagation()
  if (pointerType === 'touch' ? touchWasOpen : show.value) {
    closeTooltip()
    return
  }
  openTooltip()
}

function onDocumentClick(event: MouseEvent) {
  if (!show.value) return
  const target = event.target as Node | null
  if (!target) return
  if (triggerRef.value?.contains(target) || tooltipRef.value?.contains(target)) return
  closeTooltip()
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (!event.defaultPrevented && show.value && event.key === 'Escape' && tooltipRef.value && topFloatingSurface(tooltipRef.value)) {
    event.preventDefault()
    event.stopPropagation()
    closeTooltip()
  }
}

function onViewportChange(event?: Event) {
  if (!show.value) return
  if (event?.target instanceof Node && tooltipRef.value?.contains(event.target)) return
  if (!frame) frame = window.requestAnimationFrame(() => { frame = 0; updatePosition() })
}

function updatePosition() {
  const el = triggerRef.value
  const tooltip = tooltipRef.value
  if (!el || !tooltip) return
  const rect = el.getBoundingClientRect()
  if (!floatingAnchorVisible(el)) {
    closeTooltip()
    return
  }
  const padding = 8
  const viewport = floatingViewport(el)
  const maxWidth = viewport.width
  tooltip.style.maxWidth = `${maxWidth}px`
  tooltip.style.maxHeight = `${viewport.height}px`
  const bounds = tooltip.getBoundingClientRect()
  const width = Math.min(bounds.width, maxWidth)
  const above = Math.max(0, rect.top - viewport.top - padding)
  const below = Math.max(0, viewport.bottom - rect.bottom - padding)
  tooltipBelow.value = above < bounds.height && below > above
  const room = tooltipBelow.value ? below : above
  const center = Math.max(viewport.left + width / 2, Math.min(rect.left + rect.width / 2, viewport.right - width / 2))
  // Fixed tooltips use viewport coordinates, never document scroll offsets.
  tooltipStyle.value = {
    top: `${tooltipBelow.value ? rect.bottom + padding : rect.top - padding}px`,
    left: `${center}px`,
    maxWidth: `${maxWidth}px`,
    maxHeight: `${room}px`,
    zIndex: floatingLayer(el, 41),
    overflowY: tooltip.scrollHeight > room ? 'auto' : 'visible',
    transform: tooltipBelow.value ? 'translateX(-50%)' : 'translate(-50%, -100%)',
  }
}

function cleanup() {
  document.removeEventListener('click', onDocumentClick, true)
  window.removeEventListener('keydown', onDocumentKeydown, true)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
  window.removeEventListener('sunworld:overlay-change', onViewportChange)
  window.visualViewport?.removeEventListener('resize', onViewportChange)
  window.visualViewport?.removeEventListener('scroll', onViewportChange)
  if (frame) window.cancelAnimationFrame(frame)
  frame = 0
  observer?.disconnect()
  observer = undefined
}

watch(show, async (open) => {
  const current = ++generation
  cleanup()
  if (!open) return
  await nextTick()
  if (!show.value || destroyed || current !== generation) return
  updatePosition()
  document.addEventListener('click', onDocumentClick, true)
  window.addEventListener('keydown', onDocumentKeydown, true)
  window.addEventListener('resize', onViewportChange, { passive: true })
  window.addEventListener('scroll', onViewportChange, { capture: true, passive: true })
  window.addEventListener('sunworld:overlay-change', onViewportChange)
  window.visualViewport?.addEventListener('resize', onViewportChange, { passive: true })
  window.visualViewport?.addEventListener('scroll', onViewportChange, { passive: true })
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => onViewportChange())
    if (triggerRef.value) observer.observe(triggerRef.value)
    if (tooltipRef.value) observer.observe(tooltipRef.value)
  }
})

onBeforeUnmount(() => { destroyed = true; generation++; cleanup() })
watch(() => props.disabled, (disabled) => { if (disabled) closeTooltip() })

function onFocusIn() {
  if (props.trigger === 'hover' && pointerType !== 'touch' && !window.matchMedia('(hover: none)').matches) openTooltip()
}

function onFocusOut(event: FocusEvent) {
  if (isInside(triggerRef.value, event.relatedTarget) || isInside(tooltipRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onTriggerKeydown(event: KeyboardEvent) {
  if (props.disabled || event.target !== triggerRef.value || !['Enter', ' '].includes(event.key)) return
  event.preventDefault()
  show.value = !show.value
}
</script>

<template>
  <span
    ref="trigger"
    class="group relative ml-1 inline-flex items-center align-middle"
    :role="focusable ? 'button' : undefined"
    :tabindex="focusable ? 0 : undefined"
    :aria-describedby="show ? tooltipId : undefined"
    :aria-expanded="show"
    @pointerenter="pointerType = $event.pointerType"
    @pointerdown="onPointerDown"
    @focusin="onFocusIn"
    @focusout="onFocusOut"
    @keydown="onTriggerKeydown"
    @mouseenter="onEnter"
    @mouseleave="onLeave"
    @click="onClick"
  >
    <!-- Trigger Icon -->
    <slot name="trigger">
      <svg
        class="h-4 w-4 cursor-help text-gray-400 transition-colors hover:text-primary-600 dark:text-gray-500 dark:hover:text-primary-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    </slot>

    <!-- Teleport to body to escape modal overflow clipping -->
    <Teleport to="body">
      <!-- before: 伪元素向下延伸一段透明区域，盖住提示框与触发图标之间的空隙，让指针能连续移入提示框。 -->
      <div
        ref="tooltip"
        :id="tooltipId"
        v-show="show"
        role="tooltip"
        data-help-tooltip
        :class="[
          'fixed whitespace-normal break-words rounded-lg p-3 text-left text-xs leading-relaxed shadow-xl ring-1 selection:bg-primary-200 selection:text-gray-900 before:absolute before:inset-x-0 before:h-3',
          variant === 'surface' ? 'border border-gray-200 bg-white text-gray-700 ring-black/5 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200' : 'bg-gray-900 text-white ring-white/10 dark:bg-gray-800',
          tooltipBelow ? 'before:bottom-full' : 'before:top-full',
          props.widthClass,
          props.panelClass,
        ]"
        :style="tooltipStyle"
        @mouseleave="onTooltipLeave"
        @focusout="onFocusOut"
      >
        <button
          v-if="props.trigger === 'click'"
          type="button"
          class="absolute right-1.5 top-1.5 rounded p-1 text-gray-300 transition-colors hover:bg-white/10 hover:text-white"
          aria-label="Close"
          @click.stop="closeTooltip"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <slot>{{ content }}</slot>
        <div class="absolute left-1/2 h-2 w-2 -translate-x-1/2 rotate-45" :class="[tooltipBelow ? '-top-1' : '-bottom-1', variant === 'surface' ? 'bg-white dark:bg-dark-800' : 'bg-gray-900 dark:bg-gray-800']"></div>
      </div>
    </Teleport>
  </span>
</template>
