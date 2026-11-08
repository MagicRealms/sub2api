const GROUP_NUMBER = /(?:QQ群号|QQ群|群号)\s*[:：]?\s*([0-9]{5,15})/i
const QQ_URL = /https?:\/\/(?:qm\.qq\.com|qun\.qq\.com)\/[^\s<>"']*/i

/** Reuses the existing public contact setting without adding a new backend schema. */
export function parseCommunityContact(value: string = ''): { number: string; url: string } {
  const number = value.match(GROUP_NUMBER)?.[1] || ''
  const raw = value.match(QQ_URL)?.[0] || ''
  try {
    const url = new URL(raw)
    if (['qm.qq.com', 'qun.qq.com'].includes(url.hostname) && ['https:', 'http:'].includes(url.protocol)) return { number, url: url.href }
  } catch { /* An empty or incomplete invitation is not rendered as a link. */ }
  return { number, url: '' }
}

export function updateCommunityContact(value: string, number: string, url: string): string {
  const original = value.replace(GROUP_NUMBER, '').replace(QQ_URL, '').trim().replace(/\s{2,}/g, ' ')
  return [original, number.trim() ? `QQ群 ${number.trim().replace(/\D/g, '').slice(0, 15)}` : '', url.trim()].filter(Boolean).join(' ')
}
