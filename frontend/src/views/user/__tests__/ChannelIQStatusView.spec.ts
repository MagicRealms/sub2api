import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelIQStatusView from '@/views/user/ChannelIQStatusView.vue'

const { get, appStore } = vi.hoisted(() => ({
  get: vi.fn(),
  appStore: { cachedPublicSettings: { channel_iq_enabled: true }, showError: vi.fn() },
}))

vi.mock('@/api/channelIQ', () => ({ default: { get } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('user IQ visibility', () => {
  it('hides cards and updates navigation when the server stops publishing', async () => {
    get.mockResolvedValue({ visible: false, configured: false, enabled: false, results: [] })
    const wrapper = mount(ChannelIQStatusView, { global: { stubs: {
      AppLayout: defineComponent({ template: '<main><slot /></main>' }),
      Icon: true, ChannelIQPreview: true,
    } } })
    await flushPromises()
    expect(wrapper.text()).toContain('channelIQ.hidden')
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect(appStore.cachedPublicSettings.channel_iq_enabled).toBe(false)
    wrapper.unmount()
  })

  it('renders the saved test note instead of a numeric sequence', async () => {
    get.mockResolvedValue({
      visible: true, configured: true, enabled: false,
      results: [{
        id: 131, status: 'failed', model_id: 'gpt-6.1-sol', reasoning_effort: 'high', note: 'api-key smoke test',
        response_text: '', error_message: 'upstream failed', latency_ms: 10,
        started_at: '2026-11-17T12:00:00Z', finished_at: '2026-11-17T12:00:01Z', created_at: '2026-11-17T12:00:01Z',
      }],
    })
    const wrapper = mount(ChannelIQStatusView, { global: { stubs: {
      AppLayout: defineComponent({ template: '<main><slot /></main>' }),
      Icon: true, ChannelIQPreview: true,
    } } })
    await flushPromises()
    expect(wrapper.text()).toContain('api-key smoke test')
    expect(wrapper.text()).not.toContain('#131')
    wrapper.unmount()
  })
})
