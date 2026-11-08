import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'

import UsageStatsCards from '../UsageStatsCards.vue'

const messages: Record<string, string> = {
  'usage.totalRequests': 'Total Requests',
  'usage.inSelectedRange': 'in selected range',
  'usage.totalTokens': 'Total Tokens',
  'usage.in': 'In',
  'usage.out': 'Out',
  'usage.cacheTotal': 'Cache',
  'usage.cacheBreakdown': 'Cache Token Breakdown',
  'usage.cacheCreationTokensLabel': 'Cache Creation',
  'usage.cacheReadTokensLabel': 'Cache Read',
  'usage.totalCost': 'Total Cost',
  'usage.accountCost': 'Cost',
  'usage.standardCost': 'Standard',
  'usage.avgDuration': 'Avg Duration',
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

enableAutoUnmount(afterEach)

const stats = {
  total_requests: 1,
  total_input_tokens: 100,
  total_output_tokens: 50,
  total_cache_tokens: 34,
  total_cache_creation_tokens: 12,
  total_cache_read_tokens: 22,
  total_tokens: 184,
  total_cost: 0.001,
  total_actual_cost: 0.001,
  total_account_cost: 0.001,
  average_duration_ms: 250,
}

describe('UsageStatsCards', () => {
  it('shows cache token breakdown outside the clipped card', async () => {
    const wrapper = mount(UsageStatsCards, {
      attachTo: document.body,
      props: {
        stats,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Cache: 34')
    await wrapper.get('[data-testid="cache-token-details"]').trigger('mouseenter')
    const tooltip = document.body.querySelector<HTMLElement>('[role="tooltip"]')!
    expect(tooltip.parentElement).toBe(document.body)
    expect(wrapper.element.contains(tooltip)).toBe(false)
    expect(tooltip.style.display).not.toBe('none')
    const text = tooltip.textContent || ''
    expect(text).toContain('Cache Token Breakdown')
    expect(text).toContain('Cache Creation')
    expect(text).toContain('12')
    expect(text).toContain('Cache Read')
    expect(text).toContain('22')
  })

  it('keeps the cache tooltip out of the layout while it is hidden', () => {
    const wrapper = mount(UsageStatsCards, {
      attachTo: document.body,
      props: {
        stats,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const tooltip = document.body.querySelector<HTMLElement>('[role="tooltip"]')!
    expect(tooltip.style.display).toBe('none')
    expect(tooltip.parentElement).toBe(document.body)
    expect(wrapper.element.contains(tooltip)).toBe(false)
  })
})
