import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
import { resetHistory } from '@/api/admin/channelMonitor'

describe('channel monitor history reset API', () => {
  beforeEach(() => { post.mockReset(); post.mockResolvedValue({ data: { history_deleted: 4 } }) })
  it('uses the selected monitor endpoint with retry protection', async () => {
    await resetHistory(42, 'reset-history-42-operation')
    expect(post).toHaveBeenCalledWith('/admin/channel-monitors/42/reset-history', undefined, {
      headers: { 'Idempotency-Key': 'reset-history-42-operation' },
    })
  })
  it('uses the explicit global endpoint only for the global operation', async () => {
    await resetHistory(null, 'reset-all-history-operation')
    expect(post).toHaveBeenCalledWith('/admin/channel-monitors/reset-history', undefined, {
      headers: { 'Idempotency-Key': 'reset-all-history-operation' },
    })
  })
})
