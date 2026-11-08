import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ default: { get }, apiClient: { get } }))
import * as usage from '@/api/usage'
import * as adminUsage from '@/api/admin/usage'
import * as dashboard from '@/api/admin/dashboard'

describe('usage aggregate request cancellation', () => {
  beforeEach(() => get.mockReset().mockResolvedValue({ data: {} }))
  it('passes the caller signal through all six aggregate APIs', async () => {
    const signal = new AbortController().signal
    await usage.getStats({ timezone: 'Asia/Shanghai' }, undefined, { signal })
    await usage.getDashboardModels({}, { signal })
    await usage.getDashboardSnapshotV2({}, { signal })
    await adminUsage.getStats({}, { signal })
    await dashboard.getModelStats({}, { signal })
    await dashboard.getSnapshotV2({}, { signal })
    expect(get).toHaveBeenCalledTimes(6)
    for (const [, config] of get.mock.calls) expect(config.signal).toBe(signal)
  })
  it('keeps the legacy period and API-key arguments', async () => {
    await usage.getStats('today', 42)
    expect(get).toHaveBeenCalledWith('/usage/stats', { params: { period: 'today', api_key_id: 42 }, signal: undefined })
  })
})
