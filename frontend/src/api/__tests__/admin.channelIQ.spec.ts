import { beforeEach, describe, expect, it, vi } from 'vitest'
import { run } from '../admin/channelIQ'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { post } }))

describe('Channel IQ task submission', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('uses a stable idempotency key with the normal request timeout', async () => {
    const task = { id: 135, status: 'running' }
    post.mockResolvedValue({ data: task })
    expect(await run('one-button-click')).toEqual(task)
    expect(post).toHaveBeenCalledWith('/admin/channel-iq/run', undefined, {
      headers: { 'Idempotency-Key': 'one-button-click' },
    })
  })
})
