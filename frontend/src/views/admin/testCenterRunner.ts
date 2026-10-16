import { buildApiUrl } from '@/api/client'
import { ADMIN_UI_REQUEST_HEADER } from '@/api/adminUIRequest'

export interface TestRunnerAccount {
  id: number
  name: string
  platform: string
}

export interface TestRunnerEvent {
  type?: string
  text?: string
  model?: string
  success?: boolean
  error?: string
  image_url?: string
  url?: string
  mime_type?: string
}

export interface AccountTestRunOptions {
  account: TestRunnerAccount
  modelId: string
  prompt: string
  mode?: string
  signal?: AbortSignal
  onEvent?: (event: TestRunnerEvent) => void
  fetchImpl?: typeof fetch
}

export interface AccountTestRunResult {
  accountId: number
  accountName: string
  platform: string
  modelId: string
  status: 'success' | 'error' | 'aborted'
  durationMs: number
  output: string
  imageUrls: string[]
  error?: string
}

const parseEvent = (line: string): TestRunnerEvent | null => {
  const trimmed = line.trim()
  if (!trimmed.startsWith('data:')) return null
  const payload = trimmed.slice(5).trim()
  if (!payload || payload === '[DONE]') return null
  try {
    return JSON.parse(payload) as TestRunnerEvent
  } catch {
    return null
  }
}

const isAbortError = (error: unknown, signal?: AbortSignal) => {
  return signal?.aborted || (error instanceof DOMException && error.name === 'AbortError')
}

/**
 * Run one upstream account test and consume the existing admin SSE endpoint.
 * This is intentionally kept independent from Vue so the streaming contract
 * can be tested without mounting the whole admin page.
 */
export async function runAccountTest(options: AccountTestRunOptions): Promise<AccountTestRunResult> {
  const startedAt = Date.now()
  const fetchFn = options.fetchImpl ?? fetch
  const outputParts: string[] = []
  const imageUrls: string[] = []
  let completed = false
  let succeeded = false
  let failureMessage = ''

  const result = (status: AccountTestRunResult['status'], error?: string): AccountTestRunResult => ({
    accountId: options.account.id,
    accountName: options.account.name,
    platform: options.account.platform,
    modelId: options.modelId,
    status,
    durationMs: Math.max(0, Date.now() - startedAt),
    output: outputParts.join(''),
    imageUrls,
    error,
  })

  try {
    const body: Record<string, string> = {
      model_id: options.modelId,
      prompt: options.prompt,
    }
    if (options.mode) body.mode = options.mode

    const response = await fetchFn(buildApiUrl(`/admin/accounts/${options.account.id}/test`), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${localStorage.getItem('auth_token') ?? ''}`,
        'Content-Type': 'application/json',
        [ADMIN_UI_REQUEST_HEADER]: '1',
      },
      body: JSON.stringify(body),
      signal: options.signal,
    })

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }

    const reader = response.body?.getReader()
    if (!reader) throw new Error('The upstream test returned no stream')

    const decoder = new TextDecoder()
    let buffer = ''

    const consumeLine = (line: string) => {
      const event = parseEvent(line)
      if (!event) return
      options.onEvent?.(event)

      if (event.type === 'content' && event.text) outputParts.push(event.text)
      if (event.type === 'image' && (event.image_url || event.url)) {
        imageUrls.push(event.image_url || event.url || '')
      }
      if (event.type === 'test_complete') {
        completed = true
        succeeded = event.success === true
        if (!succeeded) failureMessage = event.error || 'Upstream test failed'
      }
      if (event.type === 'error') {
        failureMessage = event.error || 'Upstream test failed'
      }
    }

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split(/\r?\n/)
      buffer = lines.pop() || ''
      for (const line of lines) consumeLine(line)
    }
    buffer += decoder.decode()
    if (buffer) consumeLine(buffer)

    if (options.signal?.aborted) return result('aborted')
    if (!completed) return result('error', failureMessage || 'The upstream test ended without a completion event')
    return succeeded ? result('success') : result('error', failureMessage || 'Upstream test failed')
  } catch (error) {
    if (isAbortError(error, options.signal)) return result('aborted')
    return result('error', error instanceof Error ? error.message : 'Unknown upstream test error')
  }
}

export const isImageModel = (modelId: string, platform: string): boolean => {
  const id = modelId.toLowerCase()
  if (platform === 'openai') return id.startsWith('gpt-image-')
  if (platform === 'gemini' || platform === 'antigravity') {
    return id.startsWith('gemini-') && id.includes('-image')
  }
  if (platform === 'grok') {
    return id === 'grok-imagine' || id === 'grok-imagine-edit' || id.startsWith('grok-imagine-image')
  }
  return false
}

export const isVideoModel = (modelId: string, platform: string): boolean => {
  if (platform !== 'grok') return false
  const id = modelId.toLowerCase()
  return id.startsWith('grok-imagine-video') || id.startsWith('grok-video')
}
