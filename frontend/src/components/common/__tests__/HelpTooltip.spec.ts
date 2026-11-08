import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

describe('HelpTooltip', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('uses viewport coordinates after scrolling and keeps a narrow-screen tooltip within the edges', async () => {
    vi.stubGlobal('innerWidth', 320)
    vi.stubGlobal('innerHeight', 640)
    vi.stubGlobal('scrollY', 600)
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: 'details' } })
    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()
    vi.spyOn(trigger.element, 'getBoundingClientRect').mockReturnValue({ left: 4, top: 180, right: 20, bottom: 196, width: 16, height: 16 } as DOMRect)
    vi.spyOn(tooltip, 'getBoundingClientRect').mockReturnValue({ width: 256, height: 48 } as DOMRect)
    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.top).toBe('172px')
    expect(tooltip.style.left).toBe('136px')
    expect(tooltip.style.maxWidth).toBe('304px')
    wrapper.unmount()
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('keeps a hover tooltip open while the pointer moves between the trigger and the tooltip', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'copyable details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave', { relatedTarget: tooltip })
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: trigger.element }))
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: null }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports keyboard focus and dismisses before the containing dialog handles Escape', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: 'accessible details' } })
    const trigger = wrapper.get('[role="button"]')
    await trigger.trigger('focusin')
    await nextTick()
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(trigger.attributes('aria-describedby')).toBe(getTooltipElement().id)
    const parentEscape = vi.fn()
    document.addEventListener('keydown', parentEscape)
    await trigger.trigger('keydown', { key: 'Escape' })
    document.removeEventListener('keydown', parentEscape)
    expect(parentEscape).not.toHaveBeenCalled()
    expect(getTooltipElement().style.display).toBe('none')
    wrapper.unmount()
  })

  it('opens on a touch tap and closes on an outside tap', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList)
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: 'touch details' } })
    const trigger = wrapper.get('[role="button"]')
    await trigger.trigger('focusin')
    expect(getTooltipElement().style.display).toBe('none')
    await trigger.trigger('pointerdown', { pointerType: 'touch' })
    // Real browsers synthesize mouse events after a touch pointer event.
    await trigger.trigger('mouseenter')
    await trigger.trigger('click')
    await nextTick()
    await trigger.trigger('mouseleave')
    await nextTick()
    expect(getTooltipElement().style.display).not.toBe('none')
    document.body.click()
    await nextTick()
    expect(getTooltipElement().style.display).toBe('none')
    wrapper.unmount()
  })

  it('follows its dialog layer rather than covering every overlay globally', async () => {
    const dialog = document.createElement('div')
    dialog.setAttribute('role', 'dialog')
    dialog.style.zIndex = '120'
    document.body.appendChild(dialog)
    const wrapper = mount(HelpTooltip, { attachTo: dialog, props: { content: 'dialog details' } })
    await wrapper.get('[role="button"]').trigger('mouseenter')
    await nextTick()
    expect(getTooltipElement().parentElement).toBe(document.body)
    expect(getTooltipElement().style.zIndex).toBe('121')
    wrapper.unmount()
  })

  it('closes when navigation makes its trigger inert', async () => {
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: 'page details' } })
    const trigger = wrapper.get('[role="button"]')
    await trigger.trigger('mouseenter')
    await nextTick()
    trigger.element.setAttribute('inert', '')
    window.dispatchEvent(new Event('sunworld:overlay-change'))
    await new Promise(resolve => setTimeout(resolve, 30))
    expect(getTooltipElement().style.display).toBe('none')
    wrapper.unmount()
  })
})
