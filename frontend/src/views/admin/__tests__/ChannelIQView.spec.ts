import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelIQView from '@/views/admin/ChannelIQView.vue'
import type { ChannelIQSnapshot } from '@/api/admin/channelIQ'

const { get, save, run, setVisibility, listAccounts, appStore } = vi.hoisted(() => ({
  get: vi.fn(), save: vi.fn(), run: vi.fn(), setVisibility: vi.fn(), listAccounts: vi.fn(),
  appStore: {
    cachedPublicSettings: { channel_iq_enabled: true },
    showSuccess: vi.fn(), showError: vi.fn(),
  },
}))

vi.mock('@/api/admin', () => ({ adminAPI: {
  channelIQ: { get, save, run, setVisibility }, accounts: { list: listAccounts },
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

function snapshot(accountId = 1): ChannelIQSnapshot {
  return { visible: true, results: [], config: {
    id: 1, account_id: accountId, account_name: 'selected account', account_platform: 'openai',
    model_id: 'gpt-6.1-sol', prompt: 'saved prompt', reasoning_effort: 'high',
    interval_seconds: 3600, enabled: false,
  } }
}

function mountView() {
  return mount(ChannelIQView, { global: { stubs: {
    AppLayout: defineComponent({ template: '<main><slot /></main>' }),
    Icon: true, ChannelIQPreview: true,
    Select: defineComponent({
      props: ['modelValue', 'options'], emits: ['update:modelValue'],
      template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', options.find(o => String(o.value) === $event.target.value)?.value)"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>',
    }),
  } } })
}

async function clickRun(wrapper: ReturnType<typeof mountView>) {
  const button = wrapper.findAll('button').find(item => item.text().includes('admin.channelIQ.config.runNow'))!
  await button.trigger('click')
  await flushPromises()
}

describe('Channel IQ configuration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
    appStore.cachedPublicSettings.channel_iq_enabled = true
    get.mockResolvedValue(snapshot())
    listAccounts.mockResolvedValue({ items: [
      { id: 1, name: 'old OAuth', platform: 'openai', status: 'active' },
      { id: 2, name: 'new API key', platform: 'openai', status: 'active' },
    ] })
    save.mockImplementation(async input => ({ ...snapshot(Number(input.account_id)), config: { ...snapshot().config, ...input } }))
    run.mockResolvedValue({ id: 5, status: 'success', model_id: 'gpt-6.1-sol', response_text: '', latency_ms: 1 })
    setVisibility.mockImplementation(async visible => ({ visible }))
  })

  afterEach(() => { vi.useRealTimers() })

  it('saves the currently selected API key and model before starting a test', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('select')[0].setValue('2')
    await wrapper.get('input[type="text"]').setValue('gpt-6.1-terra')
    await clickRun(wrapper)
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ account_id: 2, model_id: 'gpt-6.1-terra' }))
    expect(run).toHaveBeenCalledOnce()
    expect(save.mock.invocationCallOrder[0]).toBeLessThan(run.mock.invocationCallOrder[0])
    wrapper.unmount()
  })

  it('does not run the old plan when saving the selection fails', async () => {
    save.mockRejectedValue(new Error('save failed'))
    const wrapper = mountView()
    await flushPromises()
    await clickRun(wrapper)
    expect(run).not.toHaveBeenCalled()
    expect(appStore.showError).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not overwrite account edits during automatic result refresh', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('select')[0].setValue('2')
    await wrapper.get('input[type="text"]').setValue('new-model')
    await vi.advanceTimersByTimeAsync(30000)
    await flushPromises()
    expect(get).toHaveBeenCalledTimes(2)
    expect(wrapper.get('input[type="text"]').element.value).toBe('new-model')
    expect((wrapper.findAll('select')[0].element as HTMLSelectElement).value).toBe('2')
    wrapper.unmount()
  })

  it('publishes visibility immediately without saving a plan or sending a probe', async () => {
    get.mockResolvedValue({ visible: true, config: null, results: [] })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[aria-labelledby="iq-visibility-label"]').trigger('click')
    await flushPromises()
    expect(setVisibility).toHaveBeenCalledWith(false)
    expect(save).not.toHaveBeenCalled()
    expect(run).not.toHaveBeenCalled()
    expect(appStore.cachedPublicSettings.channel_iq_enabled).toBe(false)
    expect(wrapper.get('[aria-labelledby="iq-visibility-label"]').attributes('aria-checked')).toBe('false')
    wrapper.unmount()
  })

  it('keeps visibility unchanged if the update fails', async () => {
    setVisibility.mockRejectedValue(new Error('write failed'))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[aria-labelledby="iq-visibility-label"]').trigger('click')
    await flushPromises()
    expect(appStore.cachedPublicSettings.channel_iq_enabled).toBe(true)
    expect(wrapper.get('[aria-labelledby="iq-visibility-label"]').attributes('aria-checked')).toBe('true')
    expect(appStore.showError).toHaveBeenCalled()
    wrapper.unmount()
  })
})
