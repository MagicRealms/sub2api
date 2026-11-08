import { afterEach, describe, expect, it, vi } from 'vitest'
import { measureEndpointLatency } from '../endpointLatency'
afterEach(() => vi.unstubAllGlobals())
describe('browser connection measurement', () => {
  it('uses the site health endpoint without sending API or login credentials', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response('{"status":"ok"}'))
    vi.stubGlobal('fetch', fetch)
    expect(await measureEndpointLatency('https://site.example/v1', new AbortController().signal)).toBeGreaterThanOrEqual(1)
    expect(fetch.mock.calls[0][0].origin).toBe('https://site.example')
    expect(fetch.mock.calls[0][0].pathname).toBe('/health')
    expect(fetch.mock.calls[0][1]).toMatchObject({ credentials: 'omit', cache: 'no-store', referrerPolicy: 'no-referrer' })
    expect(fetch.mock.calls[0][1].headers).toBeUndefined()
  })
  it('reports HTTP errors instead of presenting failed requests as successful latency', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 503 })))
    await expect(measureEndpointLatency('https://site.example', new AbortController().signal)).rejects.toThrow('503')
  })
  it.each(['javascript:alert(1)', 'https://user:pass@example.com'])('rejects invalid addresses %s', async address => {
    await expect(measureEndpointLatency(address, new AbortController().signal)).rejects.toThrow('Invalid endpoint')
  })
})
