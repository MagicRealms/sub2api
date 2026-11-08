import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it } from 'vitest'
import type { Chart, TooltipModel } from 'chart.js'
import { useChartTooltip } from '../useChartTooltip'

describe('chart tooltip portal', () => {
  it('escapes overflow cards, clamps to the viewport, and closes on scroll and unmount', () => {
    let show!: ReturnType<typeof useChartTooltip>
    const wrapper = mount(defineComponent({ setup() { show = useChartTooltip(); return () => h('div', { style: 'overflow:hidden' }, h('canvas')) } }), { attachTo: document.body })
    const canvas = wrapper.get('canvas').element
    canvas.getBoundingClientRect = () => ({ left: 990, top: 740, width: 30, height: 20, right: 1020, bottom: 760, x: 990, y: 740, toJSON: () => ({}) })
    const chart = { canvas, width: 30, height: 20 } as unknown as Chart<'doughnut'>
    const tooltip = { opacity: 1, caretX: 25, caretY: 15, title: ['<img src=x>'], body: [{ before: [], lines: ['Model: $1 (50%)'], after: [] }] } as unknown as TooltipModel<'doughnut'>
    show({ chart, tooltip })
    const node = document.querySelector<HTMLElement>('.sw-chart-tooltip')!
    expect(node.parentElement).toBe(document.body)
    expect(node.querySelector('img')).toBeNull()
    expect(node.textContent).toContain('Model: $1')
    expect(parseFloat(node.style.left)).toBeLessThanOrEqual(window.innerWidth - 8)
    expect(parseFloat(node.style.top)).toBeLessThanOrEqual(window.innerHeight - 8)
    window.dispatchEvent(new Event('scroll'))
    expect(node.hidden).toBe(true)
    wrapper.unmount()
    expect(document.querySelector('.sw-chart-tooltip')).toBeNull()
  })
})
