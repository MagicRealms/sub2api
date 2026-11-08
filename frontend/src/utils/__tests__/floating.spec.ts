import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { floatingAnchorVisible, floatingLayer, floatingViewport, topFloatingSurface } from '../floating'

const rect = (left: number, top: number, width = 20, height = 20) =>
  ({ left, top, width, height, right: left + width, bottom: top + height }) as DOMRect
let anchor: HTMLButtonElement

beforeEach(() => {
  vi.stubGlobal('innerWidth', 390)
  vi.stubGlobal('innerHeight', 844)
  anchor = document.createElement('button')
  document.body.appendChild(anchor)
  vi.spyOn(anchor, 'getBoundingClientRect').mockReturnValue(rect(100, 200))
})
afterEach(() => { document.body.innerHTML = ''; vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('shared floating surface boundaries', () => {
  it('keeps page menus below the drawer and dialogs, and follows the owning overlay', () => {
    expect(floatingLayer(anchor)).toBe(40)
    const drawer = document.createElement('aside')
    drawer.className = 'sidebar'
    drawer.style.zIndex = '48'
    document.body.appendChild(drawer)
    drawer.appendChild(anchor)
    expect(floatingLayer(anchor)).toBe(49)
    drawer.setAttribute('role', 'dialog')
    drawer.style.zIndex = '120'
    expect(floatingLayer(anchor)).toBe(121)
    const menu = document.createElement('div')
    menu.dataset.floatingPanel = ''
    menu.style.zIndex = '121'
    document.body.appendChild(menu)
    menu.appendChild(anchor)
    expect(floatingLayer(anchor, 41)).toBe(122)
  })

  it('does not promote ordinary cells with arbitrary z-indexes above a dialog', () => {
    anchor.style.zIndex = '99999'
    expect(floatingLayer(anchor)).toBe(40)
  })

  it('rejects triggers in the inactive page or a hidden subtree', () => {
    anchor.setAttribute('inert', '')
    expect(floatingAnchorVisible(anchor)).toBe(false)
    anchor.removeAttribute('inert')
    anchor.setAttribute('aria-hidden', 'true')
    expect(floatingAnchorVisible(anchor)).toBe(false)
  })

  it('reserves the sticky header only for page surfaces', () => {
    const header = document.createElement('header')
    header.className = 'sunworld-header'
    vi.spyOn(header, 'getBoundingClientRect').mockReturnValue(rect(0, 10, 390, 64))
    document.body.appendChild(header)
    expect(floatingViewport(anchor).top).toBe(82)
    vi.mocked(anchor.getBoundingClientRect).mockReturnValue(rect(100, 20))
    expect(floatingAnchorVisible(anchor)).toBe(false)
    header.appendChild(anchor)
    expect(floatingViewport(anchor).top).toBe(8)
    expect(floatingAnchorVisible(anchor)).toBe(true)
  })

  it('dismisses a trigger clipped out of a scroll container', () => {
    const scroll = document.createElement('div')
    scroll.style.overflowY = 'hidden'
    vi.spyOn(scroll, 'getBoundingClientRect').mockReturnValue(rect(0, 0, 390, 100))
    document.body.appendChild(scroll)
    scroll.appendChild(anchor)
    expect(floatingAnchorVisible(anchor)).toBe(false)
  })

  it('bounds surfaces to the visible mobile viewport when the keyboard is open', () => {
    vi.stubGlobal('visualViewport', { width: 360, height: 400, offsetTop: 240, offsetLeft: 15 })
    expect(floatingViewport(anchor)).toEqual({ left: 23, top: 248, right: 367, bottom: 632, width: 344, height: 384 })
    expect(floatingAnchorVisible(anchor)).toBe(false)
  })

  it('dismisses a detached trigger that previously had layout', () => {
    anchor.remove()
    expect(floatingAnchorVisible(anchor)).toBe(false)
  })

  it('gives Escape to a modal or a higher owned tooltip before the page menu', () => {
    const menu = document.createElement('div')
    menu.dataset.floatingPanel = ''
    menu.style.zIndex = '40'
    document.body.appendChild(menu)
    expect(topFloatingSurface(menu)).toBe(true)
    const dialog = document.createElement('div')
    dialog.setAttribute('role', 'dialog')
    dialog.style.zIndex = '50'
    document.body.appendChild(dialog)
    expect(topFloatingSurface(menu)).toBe(false)
    const tooltip = document.createElement('div')
    tooltip.dataset.helpTooltip = ''
    tooltip.style.zIndex = '51'
    document.body.appendChild(tooltip)
    expect(topFloatingSurface(tooltip)).toBe(true)
    tooltip.style.display = 'none'
    dialog.remove()
    expect(topFloatingSurface(menu)).toBe(true)
  })
})
