import { describe, expect, it, vi } from 'vitest'
import {
  isTestChatModel, isTestImageModel, keyUnavailableReason, listTestModels,
  runChatTest, runImageTest, supportsTestImages,
} from '../testCenter'
import type { ApiKey } from '@/types'

const key = { status: 'active', group_id: 1, group: { status: 'active', platform: 'openai', allow_image_generation: true }, quota: 0, quota_used: 0 } as ApiKey
const opts = { apiKey: 'sk-selected-user-key', model: 'gpt-chat', messages: [{ role: 'user' as const, content: '你好' }] }
const delta = (content: string) => ({ choices: [{ index: 0, delta: { content }, finish_reason: null }] })
const end = (reason = 'stop') => ({ choices: [{ index: 0, delta: {}, finish_reason: reason }] })
const frame = (event: unknown) => `data: ${JSON.stringify(event)}\r\n\r\n`

function sse(text: string, close = true) {
  const bytes = new TextEncoder().encode(text)
  const cancel = vi.fn()
  const stream = new ReadableStream({
    start(controller) {
      // One byte per chunk exercises UTF-8 and every possible SSE boundary.
      for (const byte of bytes) controller.enqueue(new Uint8Array([byte]))
      if (close) controller.close()
    }, cancel,
  })
  return { response: new Response(stream, { headers: { 'content-type': 'text/event-stream', 'x-request-id': 'request-42' } }), cancel }
}

describe('user test center gateway client', () => {
  it('discovers only models returned for the selected API key, without session credentials', async () => {
    localStorage.setItem('auth_token', 'login-jwt-must-not-be-sent')
    const fetchImpl = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [{ id: 'z' }, { id: 'a' }, { id: 'z' }, {}] })))
    expect(await listTestModels({ ...opts, fetchImpl })).toEqual(['a', 'z'])
    expect(fetchImpl).toHaveBeenCalledWith(expect.stringMatching(/\/v1\/models$/), expect.objectContaining({
      method: 'GET', credentials: 'omit', redirect: 'error', cache: 'no-store',
      headers: { Authorization: 'Bearer sk-selected-user-key', 'Content-Type': 'application/json' },
    }))
    expect(JSON.stringify(fetchImpl.mock.calls)).not.toContain('login-jwt')
    expect(JSON.stringify(fetchImpl.mock.calls)).not.toContain('/admin/')
  })

  it('streams split UTF-8 text, preserves conversation, usage and a terminal result', async () => {
    const { response } = sse(': keepalive\r\n\r\n' + frame(delta('你好')) + frame(delta('！')) + frame(end()) + frame({ choices: [], usage: { prompt_tokens: 7, completion_tokens: 3 } }) + 'data: [DONE]\r\n\r\n')
    const fetchImpl = vi.fn().mockResolvedValue(response)
    const onContent = vi.fn()
    const result = await runChatTest({ ...opts, fetchImpl, onContent })
    expect(result).toMatchObject({ content: '你好！', finishReason: 'stop', requestId: 'request-42', usage: { inputTokens: 7, outputTokens: 3 } })
    expect(onContent.mock.calls).toEqual([['你好'], ['你好！']])
    const body = JSON.parse(fetchImpl.mock.calls[0][1].body)
    expect(body.messages).toEqual(opts.messages)
    expect(body.stream).toBe(true)
    expect(fetchImpl.mock.calls[0][0]).toMatch(/\/v1\/chat\/completions$/)
  })

  it('finishes on DONE without waiting for a permanently open stream', async () => {
    const { response, cancel } = sse(frame(delta('OK')) + frame(end()) + 'data: [DONE]\n\n', false)
    expect((await runChatTest({ ...opts, fetchImpl: vi.fn().mockResolvedValue(response) })).content).toBe('OK')
    expect(cancel).toHaveBeenCalled()
  })

  it.each([
    [frame(delta('partial')), 'incompleteStream'],
    [frame(delta('partial')) + frame(end()), 'incompleteStream'],
    [frame(delta('partial')) + 'data: [DONE]\n\n', 'incompleteStream'],
    [frame(end()) + 'data: [DONE]\n\n', 'emptyReply'],
    ['data: invalid json\n\n', 'invalidResponse'],
    [frame(end('content_filter')) + 'data: [DONE]\n\n', 'contentFiltered'],
  ])('does not report an incomplete or invalid response as success (%s)', async (text, code) => {
    await expect(runChatTest({ ...opts, fetchImpl: vi.fn().mockResolvedValue(sse(text).response) })).rejects.toMatchObject({ code, requestId: 'request-42' })
  })

  it('preserves HTTP and in-stream policy errors and redacts echoed credentials', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: `Denied ${opts.apiKey}` } }), { status: 403 }))
    await expect(runChatTest({ ...opts, fetchImpl })).rejects.toMatchObject({ code: 'httpError', message: 'HTTP 403 · Denied [redacted]' })
    await expect(runChatTest({ ...opts, fetchImpl: vi.fn().mockResolvedValue(sse(frame({ error: { message: 'rate limited' } })).response) })).rejects.toMatchObject({ message: 'rate limited' })
  })

  it('does not accept a JSON body with HTTP 200 as a completed stream', async () => {
    await expect(runChatTest({ ...opts, fetchImpl: vi.fn().mockResolvedValue(new Response('{}', { headers: { 'content-type': 'application/json' } })) })).rejects.toMatchObject({ code: 'invalidResponse' })
  })

  it('distinguishes output length limits and user cancellation', async () => {
    expect((await runChatTest({ ...opts, fetchImpl: vi.fn().mockResolvedValue(sse(frame(delta('limited')) + frame(end('length')) + 'data: [DONE]\n\n').response) })).finishReason).toBe('length')
    const controller = new AbortController()
    await expect(runChatTest({ ...opts, signal: controller.signal, onContent: () => controller.abort(), fetchImpl: vi.fn().mockResolvedValue(sse(frame(delta('partial')) + frame(end()) + 'data: [DONE]\n\n').response) })).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('generates one image using the selected key through the billed gateway', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [{ b64_json: 'aGVsbG8=', revised_prompt: 'A cat' }] }), { headers: { 'x-request-id': 'image-7' } }))
    const result = await runImageTest({ ...opts, model: 'gpt-image-2', prompt: 'cat', fetchImpl })
    expect(result.images).toEqual([{ src: 'data:image/png;base64,aGVsbG8=', revisedPrompt: 'A cat' }])
    expect(result.requestId).toBe('image-7')
    expect(fetchImpl).toHaveBeenCalledWith(expect.stringMatching(/\/v1\/images\/generations$/), expect.objectContaining({
      headers: { Authorization: 'Bearer sk-selected-user-key', 'Content-Type': 'application/json' },
      body: JSON.stringify({ model: 'gpt-image-2', prompt: 'cat', n: 1, response_format: 'b64_json' }),
    }))
  })

  it.each(['javascript:alert(1)', 'data:text/html;base64,YQ==', 'data:image/svg+xml;base64,YQ==', 'https://user:password@example.com/image.png', '//example.com/image.png'])('rejects unsafe image links (%s)', async url => {
    await expect(runImageTest({ ...opts, prompt: 'cat', fetchImpl: vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [{ url }] }))) })).rejects.toMatchObject({ code: 'emptyImage' })
  })

  it('accepts safe image URLs and surfaces image permission errors', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [{ url: 'https://example.com/cat.png' }] })))
    expect((await runImageTest({ ...opts, prompt: 'cat', fetchImpl })).images[0].src).toBe('https://example.com/cat.png')
    await expect(runImageTest({ ...opts, prompt: 'cat', fetchImpl: vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: 'Image generation disabled' } }), { status: 403 })) })).rejects.toMatchObject({ message: 'HTTP 403 · Image generation disabled' })
  })
})

describe('key availability and model capabilities', () => {
  it('excludes inactive, expired, exhausted and unavailable-group keys', () => {
    expect(keyUnavailableReason(key)).toBeNull()
    expect(keyUnavailableReason({ ...key, status: 'inactive' })).toBe('keyInactive')
    expect(keyUnavailableReason({ ...key, expires_at: '2000-01-01T00:00:00Z' })).toBe('keyExpired')
    expect(keyUnavailableReason({ ...key, quota: 10, quota_used: 10 })).toBe('keyQuotaExhausted')
    expect(keyUnavailableReason({ ...key, group_id: null })).toBe('keyGroupUnavailable')
    expect(keyUnavailableReason({ ...key, group: { ...key.group!, status: 'inactive' } })).toBe('keyGroupUnavailable')
  })

  it('enforces image permission in addition to protocol capability', () => {
    expect(supportsTestImages(key)).toBe(true)
    expect(supportsTestImages({ ...key, group: { ...key.group!, allow_image_generation: false } })).toBe(false)
    expect(supportsTestImages({ ...key, group: { ...key.group!, platform: 'gemini' } })).toBe(false)
    for (const model of ['gpt-image-2', 'grok-imagine-image', 'gemini-3-pro-image-preview']) {
      expect(isTestImageModel(model)).toBe(true)
      expect(isTestChatModel(model)).toBe(false)
    }
    for (const model of ['grok-imagine-video', 'grok-imagine-edit', 'text-embedding-3-small', 'whisper-1']) expect(isTestChatModel(model)).toBe(false)
    expect(isTestChatModel('gpt-6.1-sol')).toBe(true)
    expect(isTestChatModel('claude-sonnet-4-6')).toBe(true)
  })
})
