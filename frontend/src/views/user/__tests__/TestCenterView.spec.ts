import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import zh from '@/i18n/locales/zh/testCenter'
import TestCenterView from '../TestCenterView.vue'
import type { ApiKey } from '@/types'
import type { ChatTestResult } from '@/api/testCenter'

const { listKeys, listModels, runChat, runImage } = vi.hoisted(() => ({ listKeys: vi.fn(), listModels: vi.fn(), runChat: vi.fn(), runImage: vi.fn() }))
vi.mock('@/api/keys', () => ({ keysAPI: { list: listKeys } }))
vi.mock('@/api/testCenter', async () => ({ ...await vi.importActual<typeof import('@/api/testCenter')>('@/api/testCenter'), listTestModels: listModels, runChatTest: runChat, runImageTest: runImage }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), onBeforeRouteLeave: vi.fn() }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
  const message = key.split('.').reduce((value: any, part) => value?.[part], zh)
  return typeof message === 'string' ? message.replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? name)) : key
} }) }))

const imageKey = { id: 1, user_id: 7, name: 'My image key', key: 'sk-my-image-key', status: 'active', group_id: 6, group: { id: 6, name: 'Image group', status: 'active', platform: 'openai', allow_image_generation: true }, quota: 0, quota_used: 0 } as ApiKey
const chatKey = { ...imageKey, id: 2, name: 'My chat key', key: 'sk-my-chat-key', group: { ...imageKey.group!, id: 10, name: 'Text group', allow_image_generation: false } } as ApiKey
const result: ChatTestResult = { content: 'Hello', finishReason: 'stop', durationMs: 100, firstTokenMs: 50, requestId: 'req-chat', usage: { inputTokens: 10, outputTokens: 2 } }
function render() {
  return mount(TestCenterView, { global: {
    stubs: { AppLayout: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }), Icon: true, RouterLink: { template: '<a><slot /></a>' } },
  } })
}

beforeEach(() => {
  vi.clearAllMocks()
  listKeys.mockResolvedValue({ items: [imageKey, chatKey], total: 2 })
  listModels.mockResolvedValue(['gpt-6.1-sol', 'gpt-image-2', 'gpt-chat-other'])
  runChat.mockResolvedValue(result)
  runImage.mockResolvedValue({ images: [{ src: 'data:image/png;base64,aGVsbG8=' }], durationMs: 100, requestId: 'req-image' })
})

describe('user test center', () => {
  it('loads all pages of owned keys, masks no credential into the DOM and selects per-key models', async () => {
    listKeys.mockResolvedValueOnce({ items: [imageKey], total: 2 }).mockResolvedValueOnce({ items: [chatKey], total: 2 })
    const wrapper = render()
    await flushPromises()
    expect(listKeys).toHaveBeenCalledTimes(2)
    expect(listKeys.mock.calls[1][0]).toBe(2)
    expect(listModels).toHaveBeenCalledWith(expect.objectContaining({ apiKey: imageKey.key }))
    expect(wrapper.get('[data-test="test-key"]').findAll('option').map(option => option.text())).toContain('My image key · #1')
    expect(wrapper.html()).not.toContain(imageKey.key)
    expect(wrapper.get('[data-test="test-key-info"]').text()).toContain('Image group')
    expect(wrapper.get('[data-test="test-model"]').findAll('option').map(option => option.text())).not.toContain('gpt-image-2')
    expect(wrapper.text()).toContain('正常计费')
    wrapper.unmount()
  })

  it('sends a real conversation history with only the selected key and clears it on model/key changes', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('First question')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(runChat).toHaveBeenCalledWith(expect.objectContaining({ apiKey: imageKey.key, model: 'gpt-6.1-sol', messages: [{ role: 'user', content: 'First question' }] }))
    expect(wrapper.get('[data-test="test-message-assistant"]').text()).toContain('Hello')
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('Follow up')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(runChat.mock.calls[1][0].messages).toEqual([{ role: 'user', content: 'First question' }, { role: 'assistant', content: 'Hello' }, { role: 'user', content: 'Follow up' }])
    await wrapper.get('[data-test="test-model"]').setValue('gpt-chat-other')
    expect(wrapper.findAll('[data-test="test-message-assistant"]')).toHaveLength(0)
    await wrapper.get('[data-test="test-key"]').setValue('2')
    await flushPromises()
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('New key')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(runChat.mock.calls[2][0]).toMatchObject({ apiKey: chatKey.key, messages: [{ role: 'user', content: 'New key' }] })
    wrapper.unmount()
  })

  it('cancels an active stream and excludes the partial reply from the next request', async () => {
    runChat.mockImplementationOnce(async (options) => {
      options.onContent('Partial reply')
      await new Promise((_, reject) => options.signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError'))))
    })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('Stop me')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-test="test-key"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="test-message-assistant"]').text()).toContain('Partial reply')
    await wrapper.findAll('button').find(button => button.text() === '停止')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('已停止接收')
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('Next')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(runChat.mock.calls[1][0].messages).toEqual([{ role: 'user', content: 'Next' }])
    wrapper.unmount()
  })

  it('displays failures and excludes failed turns from subsequent context', async () => {
    runChat.mockRejectedValueOnce(new Error('quota exceeded'))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('Failed turn')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('quota exceeded')
    await wrapper.get('[data-test="test-chat-prompt"]').setValue('Retry')
    await wrapper.get('[data-test="test-chat-send"]').trigger('submit')
    await flushPromises()
    expect(runChat.mock.calls[1][0].messages).toEqual([{ role: 'user', content: 'Retry' }])
    wrapper.unmount()
  })

  it('supports image previews and prevents image requests when the key group disallows them', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#test-image-tab').trigger('click')
    expect((wrapper.get('[data-test="test-model"]').element as HTMLSelectElement).value).toBe('gpt-image-2')
    await wrapper.get('[data-test="test-image-prompt"]').setValue('A cat')
    await wrapper.get('[data-test="test-image-send"]').trigger('submit')
    await flushPromises()
    expect(runImage).toHaveBeenCalledWith(expect.objectContaining({ apiKey: imageKey.key, model: 'gpt-image-2', prompt: 'A cat' }))
    expect(wrapper.get('[data-test="test-image-result"] img').attributes('src')).toContain('data:image/png')
    await wrapper.get('[data-test="test-key"]').setValue('2')
    await flushPromises()
    expect(wrapper.get('[data-test="test-image-permission"]').text()).toContain('未开放生图')
    expect(wrapper.get('[data-test="test-image-send"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="test-image-result"]').exists()).toBe(false)
    await wrapper.get('[data-test="test-image-send"]').trigger('submit')
    expect(runImage).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('ignores stale model discovery after a key switch', async () => {
    let resolveFirst!: (models: string[]) => void
    listModels.mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve }))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-test="test-key"]').setValue('2')
    await flushPromises()
    resolveFirst(['wrong-private-model'])
    await flushPromises()
    expect(wrapper.html()).not.toContain('wrong-private-model')
    expect(wrapper.get('[data-test="test-key-info"]').text()).toContain('Text group')
    wrapper.unmount()
  })

  it('offers key management when no usable key exists and never starts generation', async () => {
    listKeys.mockResolvedValue({ items: [{ ...imageKey, status: 'inactive' }], total: 1 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('暂无可用密钥')
    expect(wrapper.get('[data-test="test-chat-send"]').attributes('disabled')).toBeDefined()
    expect(listModels).not.toHaveBeenCalled()
    expect(runChat).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not send Enter during Chinese IME composition, and aborts on unmount', async () => {
    let signal: AbortSignal | undefined
    runChat.mockImplementationOnce(async (options) => {
      signal = options.signal
      await new Promise((_, reject) => signal!.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError'))))
    })
    const wrapper = render()
    await flushPromises()
    const input = wrapper.get('[data-test="test-chat-prompt"]')
    await input.setValue('你好')
    await input.trigger('keydown', { key: 'Enter', isComposing: true })
    expect(runChat).not.toHaveBeenCalled()
    await input.trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(runChat).not.toHaveBeenCalled()
    await input.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(runChat).toHaveBeenCalledTimes(1)
    wrapper.unmount()
    expect(signal?.aborted).toBe(true)
    await flushPromises()
  })
})
