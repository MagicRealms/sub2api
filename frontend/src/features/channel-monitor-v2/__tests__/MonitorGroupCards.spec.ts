import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorGroupCard, MonitorHealth } from '@/api/channelMonitorV2'
import messages from '@/i18n/locales/zh/channelMonitorV2'
import MonitorGroupCards from '../MonitorGroupCards.vue'

const coverage = { requested_start: '2026-11-08T00:00:00Z', requested_end: '2026-11-08T01:30:00Z', bucket_seconds: 60 } as MonitorCoverage
const state = (overall: string, ttft = overall): MonitorHealth => ({ overall, error_rate: overall, ttft, cache: overall, minimum_sample: 50, sample_limited: true }) as MonitorHealth
function card(groupId: number): MonitorGroupCard {
  const metrics = { error_rate: 0, cache_rate: .8, ttft: { p50_ms: 1000, p95_ms: 2000, avg_ms: 1100 }, duration: {} }
  return { group_id: groupId, group_name: `Group ${groupId}`, platform: 'openai', model: 'gpt-6.1-sol', has_traffic: true, has_cache_samples: true, health: state('healthy'), metrics,
    buckets: ['healthy', 'warning', 'critical'].map((s, i) => ({ bucket_start: new Date(Date.parse(coverage.requested_start) + i * 180000).toISOString(), health: state(s), metrics })) } as MonitorGroupCard
}
function render(cards: MonitorGroupCard[]) {
  return mount(MonitorGroupCards, { props: { cards, coverage, healthMode: 'overall', rangeLabel: '90分钟' }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: messages } })], stubs: { FloatingPanel: true } } })
}
describe('MonitorGroupCards observed health', () => {
  it('renders red/yellow/green observations and keeps unsampled slots gray', () => {
    const wrapper = render([card(6)])
    expect(wrapper.findAll('.group-history .health-healthy')).toHaveLength(1)
    expect(wrapper.findAll('.group-history .health-warning')).toHaveLength(1)
    expect(wrapper.findAll('.group-history .health-critical')).toHaveLength(1)
    expect(wrapper.findAll('.group-history .health-unknown')).toHaveLength(27)
    expect(wrapper.text()).toContain('少量样本')
  })
  it('keeps a missing first-token signal gray when changing dimensions', async () => {
    const data = card(6)
    data.buckets[0]!.health.ttft = 'unknown'
    const wrapper = render([data])
    await wrapper.setProps({ healthMode: 'ttft' })
    expect(wrapper.findAll('.group-history .health-unknown')).toHaveLength(28)
    expect(wrapper.findAll('.group-history .health-critical')).toHaveLength(1)
  })
  it('shows group glyphs with stable colors independent of health or order', async () => {
    const a = card(6), b = card(16)
    const wrapper = render([a, b])
    const colors = wrapper.findAll('.group-icon').map(x => x.classes().join(' '))
    expect(colors[0]).not.toBe(colors[1])
    expect(wrapper.findAll('.group-icon svg path')).toHaveLength(2)
    a.health = state('critical')
    await wrapper.setProps({ cards: [b, a] })
    expect(wrapper.findAll('.group-icon').map(x => x.classes().join(' '))).toEqual([...colors].reverse())
  })
  it('does not fabricate success percentages or a healthy dot for empty groups', () => {
    const data = card(6)
    data.has_traffic = false; data.has_cache_samples = false; data.model = ''; data.buckets = []
    data.health = state('unknown')
    const wrapper = render([data])
    expect(wrapper.findAll('dd').map(x => x.text())).toEqual(['—', '—', '—'])
    expect(wrapper.find('header .status-dot').classes()).toContain('health-unknown')
    expect(wrapper.findAll('.group-history .health-unknown')).toHaveLength(30)
  })
})
