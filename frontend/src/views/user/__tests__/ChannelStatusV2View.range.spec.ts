import { enableAutoUnmount, shallowMount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import { getDimensions, getGroupCards, getSnapshot } from '@/api/channelMonitorV2'
import zhChannelMonitor from '@/i18n/locales/zh/channelMonitorV2'
import zhCommon from '@/i18n/locales/zh/common'
import ChannelStatusV2View from '../ChannelStatusV2View.vue'

vi.mock('@/api/channelMonitorV2', () => ({
  getDimensions: vi.fn().mockResolvedValue({ platforms: [], groups: [], models: [] }),
  getGroupCards: vi.fn().mockResolvedValue(null),
  getSnapshot: vi.fn().mockResolvedValue(null),
}))

enableAutoUnmount(afterEach)
afterEach(() => vi.clearAllMocks())

it.each([
  ['/monitor', '7d'],
  ['/monitor?range=90m', '90m'],
  ['/monitor?range=24h', '24h'],
  ['/monitor?range=30d', '30d'],
])('opens %s with the %s range', async (path, range) => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/monitor', component: ChannelStatusV2View }],
  })
  await router.push(path)
  const i18n = createI18n({
    legacy: false,
    locale: 'zh',
    messages: { zh: { ...zhCommon, ...zhChannelMonitor } },
  })
  const wrapper = shallowMount(ChannelStatusV2View, {
    global: { plugins: [createPinia(), router, i18n], renderStubDefaultSlot: true },
  })

  expect(wrapper.get('[aria-label="时间范围"] .tab-active').text()).toBe(range)
  for (const load of [getDimensions, getSnapshot, getGroupCards]) {
    expect(load).toHaveBeenCalledWith({ range, platforms: [], groupIds: [], models: [] }, false, expect.any(AbortSignal))
  }
})
