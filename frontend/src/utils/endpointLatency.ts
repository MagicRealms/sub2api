// Browser-to-site HTTP round trip, with no API credential or model request.
export async function measureEndpointLatency(endpoint: string, signal: AbortSignal): Promise<number> {
  const url = new URL(endpoint, window.location.origin)
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password) throw new Error('Invalid endpoint')
  url.pathname = '/health'
  url.search = `latency=${Date.now()}`
  url.hash = ''
  const start = performance.now()
  const response = await fetch(url, { signal, cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer' })
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  await response.arrayBuffer()
  return Math.max(1, Math.round(performance.now() - start))
}
