import { buildGatewayUrl } from './url'
import type { ApiKey } from '@/types'

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface TestUsage {
  inputTokens: number
  outputTokens: number
}

export interface ChatTestResult {
  content: string
  finishReason: string
  durationMs: number
  firstTokenMs?: number
  requestId: string
  usage?: TestUsage
}

export interface ImageTestResult {
  images: Array<{ src: string; revisedPrompt?: string }>
  durationMs: number
  requestId: string
}

export class TestCenterError extends Error {
  constructor(public code: string, message = '', public requestId = '') {
    super(message)
    this.name = 'TestCenterError'
  }
}

type RequestOptions = { apiKey: string; signal?: AbortSignal; fetchImpl?: typeof fetch }

function gatewayFetch(path: string, options: RequestOptions, body?: unknown): Promise<Response> {
  // Deliberately bypass the dashboard Axios client: gateway requests authenticate
  // with the selected user's API key, never the login JWT or an administrator key.
  return (options.fetchImpl ?? fetch)(buildGatewayUrl(path), {
    method: body === undefined ? 'GET' : 'POST',
    headers: { Authorization: `Bearer ${options.apiKey}`, 'Content-Type': 'application/json' },
    credentials: 'omit',
    redirect: 'error',
    cache: 'no-store',
    referrerPolicy: 'no-referrer',
    signal: options.signal,
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
}

function requestId(response: Response): string {
  return response.headers.get('x-request-id') || response.headers.get('x-client-request-id') || ''
}

function gatewayError(payload: Record<string, any>, options: RequestOptions, id: string, status?: number): TestCenterError {
  const detail = payload.error ?? payload
  const message = typeof detail === 'string' ? detail : detail?.message
  // An upstream error must not accidentally echo the selected credential into UI.
  const safeMessage = typeof message === 'string' ? message.split(options.apiKey).join('[redacted]') : ''
  return new TestCenterError(status ? 'httpError' : 'requestFailed', status ? `HTTP ${status}${safeMessage ? ` · ${safeMessage}` : ''}` : safeMessage, id)
}

async function readJSON(response: Response, options: RequestOptions): Promise<Record<string, any>> {
  let payload: Record<string, any>
  try {
    payload = await response.json()
  } catch {
    throw new TestCenterError(response.ok ? 'invalidResponse' : 'httpError', response.ok ? '' : `HTTP ${response.status}`, requestId(response))
  }
  if (!payload || typeof payload !== 'object') throw new TestCenterError('invalidResponse', '', requestId(response))
  if (!response.ok || payload.error) throw gatewayError(payload, options, requestId(response), response.ok ? undefined : response.status)
  return payload
}

export async function listTestModels(options: RequestOptions): Promise<string[]> {
  const payload = await readJSON(await gatewayFetch('/v1/models', options), options)
  if (!Array.isArray(payload.data)) throw new TestCenterError('invalidResponse')
  return [...new Set<string>(payload.data.flatMap((item: { id?: unknown }) => typeof item?.id === 'string' && item.id.trim() ? [item.id] : []))].sort()
}

export function keyUnavailableReason(key: ApiKey, now = Date.now()): string | null {
  if (key.status !== 'active') return key.status === 'expired' ? 'keyExpired' : key.status === 'quota_exhausted' ? 'keyQuotaExhausted' : 'keyInactive'
  if (key.expires_at && Date.parse(key.expires_at) <= now) return 'keyExpired'
  if (key.quota > 0 && key.quota_used >= key.quota) return 'keyQuotaExhausted'
  if (!key.group_id || !key.group || key.group.status !== 'active') return 'keyGroupUnavailable'
  return null
}

export function isTestImageModel(model: string): boolean {
  return /^(gpt-image-|dall-e-)/i.test(model) || /^grok-imagine(?:-image.*)?$/i.test(model) || /^gemini-.*image/i.test(model)
}

export function isTestChatModel(model: string): boolean {
  return !isTestImageModel(model) && !/(?:embed|whisper|\btts\b|\baudio\b|realtime|moderation|transcri|\bvideo\b|^sora|grok-imagine)/i.test(model)
}

export function supportsTestImages(key: ApiKey): boolean {
  return key.group?.allow_image_generation === true && ['openai', 'grok', 'composite'].includes(key.group.platform)
}

/** Consume complete SSE frames, including split UTF-8, CRLF and multiline data. */
export async function runChatTest(options: RequestOptions & {
  model: string
  messages: ChatMessage[]
  onContent?: (content: string) => void
}): Promise<ChatTestResult> {
  const started = performance.now()
  const response = await gatewayFetch('/v1/chat/completions', options, {
    model: options.model,
    messages: options.messages,
    stream: true,
    stream_options: { include_usage: true },
    max_tokens: 2048,
  })
  const id = requestId(response)
  if (!response.ok) await readJSON(response, options)
  if (!response.headers.get('content-type')?.includes('text/event-stream')) {
    // Some gateways return a JSON error with HTTP 200.
    await readJSON(response, options)
    throw new TestCenterError('invalidResponse', '', id)
  }
  const reader = response.body?.getReader()
  if (!reader) throw new TestCenterError('invalidResponse', '', id)
  let content = ''
  let finishReason = ''
  let firstTokenMs: number | undefined
  let usage: TestUsage | undefined
  let finished = false
  let buffer = ''
  let data: string[] = []
  const decoder = new TextDecoder()

  function frame() {
    const raw = data.join('\n').trim()
    data = []
    if (!raw) return
    if (raw === '[DONE]') { finished = true; return }
    let event: Record<string, any>
    try { event = JSON.parse(raw) } catch { throw new TestCenterError('invalidResponse', '', id) }
    if (event.error || event.type === 'error') throw gatewayError(event, options, id)
    const choice = event.choices?.find((item: { index?: number }) => item.index === 0) ?? event.choices?.[0]
    const delta = choice?.delta?.content ?? choice?.delta?.refusal
    if (typeof delta === 'string' && delta) {
      firstTokenMs ??= Math.round(performance.now() - started)
      content += delta
      options.onContent?.(content)
    }
    if (typeof choice?.finish_reason === 'string') finishReason = choice.finish_reason
    if (event.usage) usage = { inputTokens: event.usage.prompt_tokens ?? 0, outputTokens: event.usage.completion_tokens ?? 0 }
  }

  function consume(text: string) {
    buffer += text
    let newline: number
    while (!finished && (newline = buffer.indexOf('\n')) >= 0) {
      const line = buffer.slice(0, newline).replace(/\r$/, '')
      buffer = buffer.slice(newline + 1)
      if (!line) frame()
      else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''))
    }
  }

  try {
    while (!finished) {
      const { done, value } = await reader.read()
      if (done) {
        consume(decoder.decode() + '\n\n')
        break
      }
      consume(decoder.decode(value, { stream: true }))
    }
    if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    if (!finished || !finishReason) throw new TestCenterError('incompleteStream', '', id)
    if (finishReason === 'content_filter') throw new TestCenterError('contentFiltered', '', id)
    if (!content.trim()) throw new TestCenterError('emptyReply', '', id)
    return { content, finishReason, durationMs: Math.round(performance.now() - started), firstTokenMs, requestId: id, usage }
  } finally {
    // A valid terminal frame completes the request even if the upstream keeps its
    // connection open. Never replay a failed, partial or cancelled generation.
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}

function imageSource(item: Record<string, unknown>): string {
  if (typeof item.b64_json === 'string' && /^[a-z\d+/=\r\n]+$/i.test(item.b64_json)) {
    const mime = item.b64_json.startsWith('/9j/') ? 'image/jpeg' : item.b64_json.startsWith('UklGR') ? 'image/webp' : 'image/png'
    return `data:${mime};base64,${item.b64_json}`
  }
  if (typeof item.url !== 'string') return ''
  if (/^data:image\/(?:png|jpeg|webp|gif|avif);base64,[a-z\d+/=\r\n]+$/i.test(item.url)) return item.url
  try {
    const url = new URL(item.url)
    return ['https:', 'http:'].includes(url.protocol) && !url.username && !url.password ? url.href : ''
  } catch { return '' }
}

export async function runImageTest(options: RequestOptions & { model: string; prompt: string }): Promise<ImageTestResult> {
  const started = performance.now()
  const response = await gatewayFetch('/v1/images/generations', options, {
    model: options.model, prompt: options.prompt, n: 1, response_format: 'b64_json',
  })
  const payload = await readJSON(response, options)
  const images = Array.isArray(payload.data) ? payload.data.flatMap((item: Record<string, unknown>) => {
    const src = item && imageSource(item)
    return src ? [{ src, revisedPrompt: typeof item.revised_prompt === 'string' ? item.revised_prompt : undefined }] : []
  }) : []
  if (!images.length) throw new TestCenterError('emptyImage', '', requestId(response))
  return { images, durationMs: Math.round(performance.now() - started), requestId: requestId(response) }
}
