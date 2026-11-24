import { describe, expect, it } from 'vitest'
import { normalAccountGroupIds, readAccountFinalFallback, writeAccountFinalFallback } from '../finalFallback'

describe('account final fallback configuration', () => {
  it('preserves normal membership while separating fallback targets', () => {
    const config = readAccountFinalFallback({
      account_final_fallback_enabled: true,
      account_final_fallback_group_ids: [6, 10, 6, '31', -1, 1.5]
    })
    expect(config).toEqual({ enabled: true, groupIds: [6, 10] })
    expect(normalAccountGroupIds([31, 6, 10], config)).toEqual([31])
    expect(writeAccountFinalFallback({ privacy_mode: 'set' }, config)).toEqual({
      privacy_mode: 'set', account_final_fallback_enabled: true, account_final_fallback_group_ids: [6, 10]
    })
  })

  it('disables fallback without promoting its former groups', () => {
    const config = readAccountFinalFallback({
      account_final_fallback_enabled: true,
      account_final_fallback_group_ids: [6]
    })
    const normal = normalAccountGroupIds([31, 6], config)
    const extra = writeAccountFinalFallback({}, { ...config, enabled: false })
    expect(normal).toEqual([31])
    expect(extra.account_final_fallback_group_ids).toEqual([])
    expect(extra.account_final_fallback_enabled).toBe(false)
  })
})
