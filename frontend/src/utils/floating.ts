// Shared stacking rules: page < header < menus < mobile navigation < dialogs.
// A body portal escapes card clipping, but must still respect its owning overlay.
export function floatingLayer(anchor: HTMLElement, base = 40): number {
  const owner = anchor.closest<HTMLElement>('[role="dialog"], .modal-overlay, .sidebar, .sunworld-header, [data-floating-panel], .driver-active-element')
  let layer = base
  if (owner) {
    for (let el: HTMLElement | null = owner; el && el !== document.body; el = el.parentElement) {
      const z = Number.parseInt(getComputedStyle(el).zIndex, 10)
      if (Number.isFinite(z)) layer = Math.max(layer, z + 1)
    }
  }
  return layer
}

// Escape belongs to the frontmost visible surface, before a parent dialog.
export function topFloatingSurface(surface: HTMLElement): boolean {
  const surfaces = Array.from(document.querySelectorAll<HTMLElement>('[data-floating-panel], [data-help-tooltip], .select-dropdown-portal, [role="dialog"], .modal-overlay'))
    .filter(el => !el.closest('[inert]') && getComputedStyle(el).display !== 'none' && getComputedStyle(el).visibility !== 'hidden')
  const layer = Number.parseInt(getComputedStyle(surface).zIndex, 10) || 0
  return !surfaces.some(el => el !== surface && ((Number.parseInt(getComputedStyle(el).zIndex, 10) || 0) > layer))
}

export function floatingViewport(anchor: HTMLElement, padding = 8) {
  const viewport = window.visualViewport
  const left = (viewport?.offsetLeft ?? 0) + padding
  let top = (viewport?.offsetTop ?? 0) + padding
  const right = left + (viewport?.width ?? window.innerWidth) - padding * 2
  const bottom = top + (viewport?.height ?? window.innerHeight) - padding * 2
  if (!anchor.closest('[role="dialog"], .modal-overlay, .sidebar, .sunworld-header, .driver-active-element')) {
    const header = document.querySelector<HTMLElement>('.sunworld-header')
    if (header) top = Math.max(top, header.getBoundingClientRect().bottom + padding)
  }
  return { left, top, right, bottom, width: Math.max(0, right - left), height: Math.max(0, bottom - top) }
}

export function floatingAnchorVisible(anchor: HTMLElement): boolean {
  if (anchor.closest('[inert], [aria-hidden="true"]')) return false
  const rect = anchor.getBoundingClientRect()
  // Zero geometry is useful for unlaid-out unit-test DOMs; real triggers have width.
  if (!rect.width || !rect.height) return true
  if (!anchor.isConnected) return false
  const viewport = floatingViewport(anchor, 0)
  if (rect.bottom <= viewport.top || rect.top >= viewport.bottom || rect.right <= viewport.left || rect.left >= viewport.right) return false
  for (let el = anchor.parentElement; el && el !== document.body; el = el.parentElement) {
    const style = getComputedStyle(el)
    const bounds = el.getBoundingClientRect()
    if ((/(auto|scroll|hidden|clip)/.test(style.overflowY) && (rect.bottom <= bounds.top || rect.top >= bounds.bottom)) ||
        (/(auto|scroll|hidden|clip)/.test(style.overflowX) && (rect.right <= bounds.left || rect.left >= bounds.right))) return false
  }
  return true
}
