import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import FloatingPanel from '../FloatingPanel.vue'
import { defineComponent } from 'vue'

enableAutoUnmount(afterEach)
let anchor: HTMLButtonElement
let anchorRect: DOMRect
const rect = (left: number, top: number, width: number, height: number) =>
  ({ left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON: () => ({}) }) as DOMRect

beforeEach(() => {
  vi.stubGlobal('innerWidth', 360)
  vi.stubGlobal('innerHeight', 640)
  anchor = document.createElement('button')
  document.body.appendChild(anchor)
  anchorRect = rect(320, 550, 30, 40)
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    if (this === anchor) return anchorRect
    if (this.hasAttribute('data-floating-panel')) {
      const width = Math.min(Math.max(200, Number.parseFloat(this.style.minWidth) || 0), Number.parseFloat(this.style.maxWidth) || 360)
      return rect(0, 0, width, 280)
    }
    return rect(0, 0, 360, 640)
  })
})
afterEach(() => { anchor.remove(); vi.restoreAllMocks(); vi.unstubAllGlobals(); document.querySelectorAll('[data-test-fixture]').forEach(el => el.remove()) })

async function open(props = {}) {
  const wrapper = mount(FloatingPanel, {
    props: { open: true, anchor, align: 'end', ...props },
    slots: { default: '<button>option</button>' },
    attachTo: document.body
  })
  await flushPromises()
  return { wrapper, panel: document.querySelector<HTMLElement>('[data-floating-panel]')! }
}

describe('FloatingPanel containment and layers', () => {
  it('anchors an inline search menu to its container while portaling the panel', async () => {
    const Host = defineComponent({
      components: { FloatingPanel },
      props: { open: Boolean },
      template: '<div><input /><FloatingPanel :open="open" :match-width="true"><button>result</button></FloatingPanel></div>'
    })
    const wrapper = mount(Host, { props: { open: false }, attachTo: document.body })
    vi.spyOn(wrapper.element, 'getBoundingClientRect').mockReturnValue(rect(20, 100, 140, 40))
    await wrapper.setProps({ open: true })
    await flushPromises()
    const panel = document.querySelector<HTMLElement>('[data-floating-panel]')!
    expect(panel.parentElement).toBe(document.body)
    expect(wrapper.element.contains(panel)).toBe(false)
    expect(panel.style.width).toBe('140px')
    expect(panel.style.left).toBe('20px')
    expect(panel.style.top).toBe('148px')
  })

  it('escapes clipping ancestors and flips above a trigger near the viewport bottom', async () => {
    const card = document.createElement('div')
    card.dataset.testFixture = 'card'
    card.style.overflow = 'hidden'
    document.body.appendChild(card)
    card.appendChild(anchor)
    const { panel } = await open()
    expect(panel.parentElement).toBe(document.body)
    expect(card.contains(panel)).toBe(false)
    expect(panel.style.position).toBe('fixed')
    expect(panel.style.left).toBe('150px')
    expect(panel.style.top).toBe('262px')
    expect(panel.style.zIndex).toBe('40')
  })

  it('bounds a wide panel on a narrow viewport', async () => {
    vi.stubGlobal('innerWidth', 320)
    anchorRect = rect(10, 120, 100, 32)
    const { panel } = await open({ minWidth: 400 })
    expect(panel.style.minWidth).toBe('304px')
    expect(panel.style.maxWidth).toBe('304px')
    expect(panel.style.left).toBe('8px')
  })

  it('keeps a menu above its own custom dialog without raising page menus above dialogs', async () => {
    const dialog = document.createElement('div')
    dialog.dataset.testFixture = 'dialog'
    dialog.setAttribute('role', 'dialog')
    dialog.style.zIndex = '120'
    document.body.appendChild(dialog)
    dialog.appendChild(anchor)
    const { panel } = await open()
    expect(panel.style.zIndex).toBe('121')
  })

  it('ignores interaction inside the portal and dismisses on an outside pointer', async () => {
    const { wrapper, panel } = await open()
    panel.querySelector('button')!.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    expect(wrapper.emitted('close')).toBeUndefined()
    document.body.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('handles Escape before the parent dialog and restores focus to the trigger', async () => {
    const { wrapper, panel } = await open()
    const parentEscape = vi.fn()
    document.addEventListener('keydown', parentEscape)
    const option = panel.querySelector('button')!
    option.focus()
    option.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    document.removeEventListener('keydown', parentEscape)
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(parentEscape).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(anchor)
  })

  it('closes when its trigger scrolls out of the viewport', async () => {
    const { wrapper } = await open()
    anchorRect = rect(20, -60, 100, 30)
    window.dispatchEvent(new Event('scroll'))
    await new Promise(resolve => setTimeout(resolve, 30))
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('removes viewport listeners when unmounted', async () => {
    const { wrapper } = await open()
    wrapper.unmount()
    anchorRect = rect(20, -60, 100, 30)
    window.dispatchEvent(new Event('scroll'))
    document.body.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 30))
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
