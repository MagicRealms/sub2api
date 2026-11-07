import type { ApiKey, Group } from '@/types'

/** Resolve every selected route in its saved order, including legacy keys. */
export function monitorKeyGroups(key: ApiKey, groups: Group[]): Group[] {
  const catalog = new Map(groups.map(group => [group.id, group]))
  if (key.group) catalog.set(key.group.id, key.group)
  const ids = key.group_ids?.length ? key.group_ids : key.group_id ? [key.group_id] : []
  return [...new Set(ids)].flatMap(id => catalog.has(id) ? [catalog.get(id)!] : [])
}

export function monitorKeySupportsProvider(key: ApiKey, groups: Group[], provider: string): boolean {
  return monitorKeyGroups(key, groups).some(group =>
    group.platform === provider || group.platform === 'composite',
  )
}

/** Mirror the monitor's path join without confusing host names with paths. */
export function monitorRequestURL(endpoint: string, provider: string, apiMode: string): string {
  const base = endpoint.trim().replace(/\/+$/, '')
  if (!base) return ''
  let url: URL
  try { url = new URL(base) } catch { return '' }
  let path = provider === 'anthropic' ? '/v1/messages'
    : provider === 'gemini' ? '/v1beta/models/{model}:generateContent'
    : provider === 'openai' && apiMode === 'responses' ? '/v1/responses'
    : '/v1/chat/completions'
  if (provider === 'zhipu') {
    if (url.pathname.includes('/paas/v4')) path = '/chat/completions'
    else if (/^(?:.+\.)?(?:bigmodel\.cn|z\.ai)$/.test(url.hostname)) path = '/api/paas/v4/chat/completions'
  }
  for (let end = path.lastIndexOf('/'); end > 0; end = path.lastIndexOf('/', end - 1)) {
    if (url.pathname.endsWith(path.slice(0, end))) return base + path.slice(end)
  }
  return base + path
}
