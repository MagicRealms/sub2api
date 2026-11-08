import { onBeforeUnmount, onMounted } from 'vue'
import type { Chart, ChartType, TooltipModel } from 'chart.js'
import { floatingAnchorVisible, floatingLayer, floatingViewport } from '@/utils/floating'
import '@/styles/chart-tooltip.css'

// Render outside the canvas and card. Native canvas tooltips cannot escape either.
export function useChartTooltip() {
  let element: HTMLDivElement | undefined
  const hide = () => { if (element) element.hidden = true }
  onMounted(() => {
    window.addEventListener('scroll', hide, true)
    window.addEventListener('resize', hide)
  })
  onBeforeUnmount(() => {
    element?.remove()
    window.removeEventListener('scroll', hide, true)
    window.removeEventListener('resize', hide)
  })
  return <T extends ChartType>({ chart, tooltip }: { chart: Chart<T>; tooltip: TooltipModel<T> }) => {
    if (!tooltip.opacity || !floatingAnchorVisible(chart.canvas)) { hide(); return }
    if (!element) {
      element = document.createElement('div')
      element.className = 'sw-chart-tooltip'
      element.setAttribute('role', 'tooltip')
      document.body.appendChild(element)
    }
    element.replaceChildren()
    const addLine = (text: string, strong = false) => {
      const line = document.createElement(strong ? 'strong' : 'div')
      line.textContent = text
      element!.appendChild(line)
    }
    tooltip.title?.forEach(line => addLine(line, true))
    tooltip.beforeBody?.forEach(line => addLine(line))
    tooltip.body?.forEach(body => [...body.before, ...body.lines, ...body.after].forEach(line => addLine(line)))
    tooltip.afterBody?.forEach(line => addLine(line))
    tooltip.footer?.forEach(line => addLine(line))
    element.hidden = false
    element.style.zIndex = String(floatingLayer(chart.canvas, 45))
    const viewport = floatingViewport(chart.canvas)
    element.style.maxWidth = `${Math.min(360, viewport.width)}px`
    element.style.maxHeight = `${viewport.height}px`
    const canvas = chart.canvas.getBoundingClientRect()
    const x = canvas.left + tooltip.caretX * canvas.width / chart.width
    const y = canvas.top + tooltip.caretY * canvas.height / chart.height
    const bounds = element.getBoundingClientRect()
    element.style.left = `${Math.max(viewport.left, Math.min(x + 12, viewport.right - bounds.width))}px`
    element.style.top = `${Math.max(viewport.top, Math.min(y + 12, viewport.bottom - bounds.height))}px`
  }
}
