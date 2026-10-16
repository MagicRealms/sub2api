import { beforeEach, describe, expect, it, vi } from 'vitest'
import { isImageModel, isVideoModel, runAccountTest } from '@/views/admin/testCenterRunner'

const account = { id: 7, name: 'primary', platform: 'openai' }

function streamResponse(...events: string[]) {
  const chunks = events.map((event) => new TextEncoder().encode(`data: ${JSON.stringify(event)}\n\n`))
  let index = 0
  return {
    ok: true,
    status: 200,
    body: {
      getReader: () => ({
        read: vi.fn(async () => {
          if (index >= chunks.length) return { done: true, value: undefined }
          return { done: false, value: chunks[index++] }
        }),
      }),
    },
  } as unknown as Response
}

describe('test center runner', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('auth_token', 'admin-token')
  })

  it('consumes chat content and returns a successful result', async () => {
    const fetchImpl = vi.fn(async () => streamResponse(
      { type: 'test_start', model: 'gpt-5.4' },
      { type: 'content', text: '稳定性' },
      { type: 'content', text: '测试通过' },
      { type: 'test_complete', success: true },
    ))

    const result = await runAccountTest({
      account,
      modelId: 'gpt-5.4',
      prompt: 'ping',
      fetchImpl,
    })

    expect(result.status).toBe('success')
    expect(result.output).toBe('稳定性测试通过')
    expect(result.imageUrls).toEqual([])
    expect(fetchImpl).toHaveBeenCalledWith('/api/v1/admin/accounts/7/test', expect.objectContaining({
      body: JSON.stringify({ model_id: 'gpt-5.4', prompt: 'ping' }),
    }))
  })

  it('collects image URLs and preserves an upstream failure', async () => {
    const result = await runAccountTest({
      account: { ...account, platform: 'grok' },
      modelId: 'grok-imagine-image',
      prompt: 'cat',
      mode: 'image',
      fetchImpl: vi.fn(async () => streamResponse(
        { type: 'image', image_url: 'data:image/png;base64,abc' },
        { type: 'test_complete', success: false, error: 'rate limited' },
      )),
    })

    expect(result.status).toBe('error')
    expect(result.error).toBe('rate limited')
    expect(result.imageUrls).toEqual(['data:image/png;base64,abc'])
  })

  it('recognizes supported image and video model IDs', () => {
    expect(isImageModel('gpt-image-1', 'openai')).toBe(true)
    expect(isImageModel('gemini-2.5-flash-image', 'gemini')).toBe(true)
    expect(isImageModel('grok-imagine-image', 'grok')).toBe(true)
    expect(isImageModel('gpt-5.4', 'openai')).toBe(false)
    expect(isVideoModel('grok-imagine-video', 'grok')).toBe(true)
    expect(isVideoModel('grok-imagine-image', 'grok')).toBe(false)
  })
})
