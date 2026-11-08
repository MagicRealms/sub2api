import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import TokenUsageTrend from '../TokenUsageTrend.vue'

const messages: Record<string, string> = {
  'admin.dashboard.tokenUsageTrend': 'Token Usage Trend',
  'admin.dashboard.noDataAvailable': 'No data available',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div class="chart-data" @click="options.onClick({}, [{ index: 0 }])">{{ JSON.stringify(data) }}</div>',
  },
}))

describe('TokenUsageTrend', () => {
  it('exposes weighted cache rate and unit cost by default and lets summaries switch the chart', async () => {
    const wrapper = mount(TokenUsageTrend, { props: { interactive: true, trendData: [
      { date:'2026-11-08', requests:1, input_tokens:500, output_tokens:100, cache_read_tokens:1500, cache_creation_tokens:0, total_tokens:2100, cost:5, actual_cost:2.5 },
    ] } })
    expect(wrapper.get('.sw-trend-overview').text()).toContain('75.0%')
    expect(wrapper.get('.sw-trend-overview').text()).toContain('$1190.48')
    await wrapper.findAll('.sw-trend-overview button')[0].trigger('click')
    expect(JSON.parse(wrapper.get('.chart-data').text()).datasets[0].data).toEqual([75])
  })
  it('switches between request counts and actual charges using the same data', async () => {
    const wrapper = mount(TokenUsageTrend, { props: { trendData: [
      { date: '2026-11-08', requests: 12, input_tokens: 100, output_tokens: 20, cache_read_tokens: 40, cache_creation_tokens: 0, total_tokens: 160, cost: 5, actual_cost: 2.5 }
    ] } })
    await wrapper.findAll('.sw-segmented button')[1].trigger('click')
    expect(JSON.parse(wrapper.get('.chart-data').text()).datasets[0].data).toEqual([12])
    expect(wrapper.get('.sw-chart-total').text()).toBe('12')
    await wrapper.findAll('.sw-segmented button')[2].trigger('click')
    expect(JSON.parse(wrapper.get('.chart-data').text()).datasets[0].data).toEqual([2.5])
    expect(wrapper.get('.sw-chart-total').text()).toBe('$2.50')
  })

  it('shows cache hit rate and weighted unit cost as selectable time series', async () => {
    const wrapper = mount(TokenUsageTrend, { props: { trendData: [
      { date: '2026-11-08 10:00', requests: 2, input_tokens: 500, output_tokens: 100, cache_read_tokens: 1500, cache_creation_tokens: 0, total_tokens: 2100, cost: 5, actual_cost: 2.5 },
      { date: '2026-11-08 11:00', requests: 1, input_tokens: 500, output_tokens: 100, cache_read_tokens: 0, cache_creation_tokens: 0, total_tokens: 600, cost: 1, actual_cost: 1.5 },
    ] } })

    await wrapper.findAll('.sw-segmented button')[3].trigger('click')
    let chart = JSON.parse(wrapper.get('.chart-data').text())
    expect(chart.datasets[0].data).toEqual([75, 0])
    expect(wrapper.get('.sw-chart-total').text()).toBe('60.0%')

    await wrapper.findAll('.sw-segmented button')[4].trigger('click')
    chart = JSON.parse(wrapper.get('.chart-data').text())
    expect(chart.datasets[0].data[0]).toBeCloseTo((2.5 / 2100) * 1_000_000)
    expect(wrapper.get('.sw-chart-total').text()).toBe('$1481.48')
  })

  it('emits the selected trend point when interactive chart data is clicked', async () => {
    const point = { date: '2026-11-08 10:00', requests: 2, input_tokens: 500, output_tokens: 100, cache_read_tokens: 1500, cache_creation_tokens: 0, total_tokens: 2100, cost: 5, actual_cost: 2.5 }
    const wrapper = mount(TokenUsageTrend, { props: { trendData: [point], interactive: true } })
    await wrapper.get('.chart-data').trigger('click')

    expect(wrapper.emitted('pointSelect')?.[0]).toEqual([point])
  })
  it('calculates cache hit rate against all prompt tokens', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [
          {
            date: '2026-05-08',
            requests: 1,
            input_tokens: 500,
            output_tokens: 100,
            cache_creation_tokens: 0,
            cache_read_tokens: 1500,
            cost: 0.01,
            actual_cost: 0.005,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    const hitRateDataset = chartData.datasets.find(
      (ds: any) => ds.label === 'Cache Hit Rate'
    )
    // Hit rate = 1500 / (500 + 1500 + 0) * 100 = 75%
    expect(hitRateDataset.data[0]).toBe(75)
  })

  it('returns 0 hit rate when all prompt tokens are zero', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [
          {
            date: '2026-05-08',
            requests: 0,
            input_tokens: 0,
            output_tokens: 0,
            cache_creation_tokens: 0,
            cache_read_tokens: 0,
            cost: 0,
            actual_cost: 0,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    const hitRateDataset = chartData.datasets.find(
      (ds: any) => ds.label === 'Cache Hit Rate'
    )
    expect(hitRateDataset.data[0]).toBe(0)
  })

  it('includes cache_creation_tokens in denominator for Anthropic models', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [
          {
            date: '2026-05-08',
            requests: 1,
            input_tokens: 200,
            output_tokens: 50,
            cache_creation_tokens: 300,
            cache_read_tokens: 500,
            cost: 0.02,
            actual_cost: 0.01,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    const hitRateDataset = chartData.datasets.find(
      (ds: any) => ds.label === 'Cache Hit Rate'
    )
    // Hit rate = 500 / (200 + 500 + 300) * 100 = 50%
    expect(hitRateDataset.data[0]).toBe(50)
  })
})
