import { describe, expect, it } from 'vitest'
import { parseCommunityContact, updateCommunityContact } from '../communityContact'

describe('community contact settings', () => {
  it('reads a QQ group invitation without treating a support QQ number as the group', () => {
    expect(parseCommunityContact('客服 QQ: 123456 QQ群 987654321 https://qm.qq.com/q/example')).toEqual({ number: '987654321', url: 'https://qm.qq.com/q/example' })
    expect(parseCommunityContact('客服 QQ: 123456')).toEqual({ number: '', url: '' })
  })
  it('preserves existing support details while replacing or clearing the invitation', () => {
    const original = 'support@example.com QQ群 987654321 https://qm.qq.com/q/old'
    expect(updateCommunityContact(original, '111222333', 'https://qun.qq.com/new')).toBe('support@example.com QQ群 111222333 https://qun.qq.com/new')
    expect(updateCommunityContact(original, '', '')).toBe('support@example.com')
  })
  it('does not expose arbitrary or lookalike URLs as QQ join links', () => {
    for (const url of ['javascript:alert(1)', 'https://qm.qq.com.attacker.example/q/x', 'https://example.com']) {
      expect(parseCommunityContact(`QQ群 987654321 ${url}`).url).toBe('')
    }
  })
})
