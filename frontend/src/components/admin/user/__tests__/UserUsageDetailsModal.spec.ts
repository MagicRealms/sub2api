import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UserUsageDetailsModal from '../UserUsageDetailsModal.vue'
import type { AdminUser } from '@/types'
import type { UserAPIKeyUsage, UserUsageDetails, UserUsageMetrics } from '@/api/admin/users'

const { getDetails, getModels } = vi.hoisted(() => ({ getDetails: vi.fn(), getModels: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: { getUserUsageDetails: getDetails }, dashboard: { getModelStats: getModels } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'en' }, t: (key: string) => key }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))

enableAutoUnmount(afterEach)
beforeEach(() => { getDetails.mockReset(); getModels.mockReset().mockResolvedValue({ models: [] }) })
const user = (id: number) => ({ id, email: `user${id}@example.com`, username: `user${id}`, balance: 12, concurrency: 2, status: 'active', notes: '', created_at: '2026-09-01' }) as AdminUser
const metrics = (values: Partial<UserUsageMetrics> = {}): UserUsageMetrics => ({
  requests: 0, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, actual_cost: 0, ...values
})
const dates = ['2026-10-05', '2026-10-06', '2026-10-07', '2026-10-08', '2026-10-09', '2026-10-10', '2026-10-11']
const key = (id: number): UserAPIKeyUsage => ({
  id, name: `key-${id}`, key_preview: 'sk-demo…1234', status: 'active', deleted: false, group_name: 'OpenAI', expires_at: null, last_used_at: '2026-10-11T01:00:00Z',
  total: metrics({ requests: 10, total_tokens: 1000, actual_cost: 15 }),
  daily: dates.map(date => ({ date, ...metrics(date === '2026-10-11' ? { requests: 2, total_tokens: 200, actual_cost: 2 } : date === '2026-10-10' ? { requests: 1, total_tokens: 100, actual_cost: 1 } : {}) }))
})
const response = (id = 1): UserUsageDetails => ({
  user: user(id), generated_at: '2026-10-11T03:00:00Z', start_date: dates[0], end_date: dates[6], timezone: 'Asia/Shanghai',
  total: metrics({ requests: 18, total_tokens: 1800, actual_cost: 25 }),
  daily: dates.map(date => ({ date, ...metrics(date === '2026-10-11' ? { requests: 3, total_tokens: 300, actual_cost: 4 } : date === '2026-10-10' ? { requests: 2, total_tokens: 200, actual_cost: 3 } : {}) })),
  api_keys: [key(1)]
})
const open = (show = true) => mount(UserUsageDetailsModal, {
  props: { show, user: user(1) },
  global: { stubs: {
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
    LoadingSpinner: { template: '<span data-test="loading">loading</span>' }
  } }
})
function deferred<T = UserUsageDetails>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('user usage details', () => {
  it('defaults to today followed by cumulative metrics and shows seven calendar dates', async () => {
    getDetails.mockResolvedValue(response())
    const wrapper = open(); await flushPromises()
    expect(wrapper.findAll('[data-test^="usage-day-"]')).toHaveLength(7)
    expect(wrapper.get('[data-test="usage-day-2026-10-11"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="day-requests"]').text()).toBe('3')
    expect(wrapper.get('[data-test="total-requests"]').text()).toBe('18')
    expect(wrapper.get('[data-test="day-tokens"]').text()).toBe('<0.01 M')
    expect(wrapper.get('[data-test="day-cost"]').text()).toBe('$4.00')
    expect(wrapper.get('[data-test="usage-key-1"]').text()).toContain('2 / <0.01 M')
    expect(wrapper.get('[data-test="usage-key-1"]').text()).toContain('10 / <0.01 M')
    expect(wrapper.text()).toContain('sk-demo…1234')
    expect(getDetails).toHaveBeenCalledWith(1, expect.objectContaining({ timezone: expect.any(String), signal: expect.any(AbortSignal) }))
  })

  it('switches the overview and API keys together without changing cumulative usage', async () => {
    getDetails.mockResolvedValue(response())
    const wrapper = open(); await flushPromises()
    await wrapper.get('[data-test="usage-day-2026-10-10"]').trigger('click')
    expect(wrapper.get('[data-test="day-requests"]').text()).toBe('2')
    expect(wrapper.get('[data-test="day-cost"]').text()).toBe('$3.00')
    expect(wrapper.get('[data-test="usage-key-1"]').text()).toContain('1 / <0.01 M')
    expect(wrapper.get('[data-test="total-requests"]').text()).toBe('18')
    await wrapper.get('[data-test="usage-day-2026-10-05"]').trigger('click')
    expect(wrapper.get('[data-test="day-requests"]').text()).toBe('0')
    expect(wrapper.get('[data-test="usage-key-1"]').text()).toContain('0 / 0 M')
    expect(getDetails).toHaveBeenCalledTimes(1)
  })

  it('aborts the previous request and ignores a late response after switching users', async () => {
    const old = deferred()
    getDetails.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ ...response(2), api_keys: [{ ...key(2), name: 'current-key' }] })
    const wrapper = open()
    const signal = getDetails.mock.calls[0][1].signal as AbortSignal
    await wrapper.setProps({ user: user(2) }); await flushPromises()
    expect(signal.aborted).toBe(true)
    old.resolve({ ...response(1), api_keys: [{ ...key(1), name: 'previous-key' }] }); await flushPromises()
    expect(wrapper.text()).toContain('user2@example.com')
    expect(wrapper.text()).toContain('current-key')
    expect(wrapper.text()).not.toContain('previous-key')
  })

  it('does not leak previous user data when the next request fails and can retry', async () => {
    getDetails.mockResolvedValueOnce(response(1)).mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce(response(2))
    const wrapper = open(); await flushPromises()
    await wrapper.setProps({ user: user(2) }); await flushPromises()
    expect(wrapper.text()).not.toContain('user1@example.com')
    expect(wrapper.find('[data-test="usage-key-1"]').exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('user2@example.com')
  })

  it('keeps the current loading state when an obsolete request fails', async () => {
    const old = deferred(); const current = deferred()
    getDetails.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = open()
    await wrapper.setProps({ user: user(2) })
    old.reject(new Error('obsolete')); await flushPromises()
    expect(wrapper.find('[data-test="loading"]').exists()).toBe(true)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    current.resolve(response(2)); await flushPromises()
    expect(wrapper.find('[data-test="loading"]').exists()).toBe(false)
  })

  it('paginates all keys and searches beyond the first page', async () => {
    const result = response()
    result.api_keys = Array.from({ length: 25 }, (_, i) => key(i + 1))
    getDetails.mockResolvedValue(result)
    const wrapper = open(); await flushPromises()
    expect(wrapper.findAll('[data-test^="usage-key-"]')).toHaveLength(8)
    await wrapper.get('input[type="search"]').setValue('key-25')
    expect(wrapper.findAll('[data-test^="usage-key-"]')).toHaveLength(1)
    expect(wrapper.find('[data-test="usage-key-25"]').exists()).toBe(true)
  })

  it('shows unused, deleted and expired keys with their actual status', async () => {
    const result = response()
    result.api_keys = [
      { ...key(1), name: 'unused', total: metrics(), daily: [] },
      { ...key(2), deleted: true },
      { ...key(3), expires_at: '2026-10-10T00:00:00Z' }
    ]
    getDetails.mockResolvedValue(result)
    const wrapper = open(); await flushPromises()
    expect(wrapper.get('[data-test="usage-key-1"]').text()).toContain('0 / 0')
    expect(wrapper.get('[data-test="usage-key-2"]').text()).toContain('admin.users.usageDetails.deleted')
    expect(wrapper.get('[data-test="usage-key-3"]').text()).toContain('admin.users.usageDetails.expired')
  })

  it('does not request data while closed and defaults to today when reopened', async () => {
    getDetails.mockResolvedValue(response())
    const wrapper = open(false)
    expect(getDetails).not.toHaveBeenCalled()
    expect(getModels).not.toHaveBeenCalled()
    await wrapper.setProps({ show: true }); await flushPromises()
    await wrapper.get('[data-test="usage-day-2026-10-10"]').trigger('click')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true }); await flushPromises()
    expect(wrapper.get('[data-test="usage-day-2026-10-11"]').attributes('aria-pressed')).toBe('true')
  })

  it('shows actual-cost model shares for the selected user and day in at most four rows', async () => {
    getDetails.mockResolvedValue(response())
    getModels.mockResolvedValue({ models: [
      { model: 'large', actual_cost: 8 }, { model: 'mid', actual_cost: 4 },
      { model: 'small', actual_cost: 2 }, { model: 'tail-a', actual_cost: 1 },
      { model: 'tail-b', actual_cost: 1 }, { model: 'free', actual_cost: 0 }
    ] })
    const wrapper = open(); await flushPromises()
    expect(getModels).toHaveBeenCalledWith({ user_id: 1, start_date: '2026-10-11', end_date: '2026-10-11', timezone: 'Asia/Shanghai', model_source: 'requested' }, { signal: expect.any(AbortSignal) })
    const rows = wrapper.findAll('[data-test="usage-model-row"]')
    expect(rows).toHaveLength(4)
    expect(rows[0].text()).toContain('large$8.0050.0%')
    expect(rows[1].text()).toContain('mid$4.0025.0%')
    expect(rows[2].text()).toContain('small$2.0012.5%')
    expect(rows[3].text()).toContain('admin.users.usageDetails.otherModels$2.0012.5%')
    expect(wrapper.get('[data-test="day-cost"]').text()).toBe('$4.00')
  })

  it('switches distribution dates and reuses only this open user session cache', async () => {
    getDetails.mockResolvedValue(response())
    getModels.mockResolvedValueOnce({ models: [{ model: 'today-model', actual_cost: 4 }] })
      .mockResolvedValueOnce({ models: [{ model: 'yesterday-model', actual_cost: 3 }] })
    const wrapper = open(); await flushPromises()
    await wrapper.get('[data-test="usage-day-2026-10-10"]').trigger('click'); await flushPromises()
    expect(getModels.mock.calls[1][0]).toEqual(expect.objectContaining({ user_id: 1, start_date: '2026-10-10', end_date: '2026-10-10' }))
    expect(wrapper.get('[data-test="usage-model-consumption"]').text()).toContain('yesterday-model')
    expect(wrapper.get('[data-test="usage-model-consumption"]').text()).not.toContain('today-model')
    await wrapper.get('[data-test="usage-day-2026-10-11"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[data-test="usage-model-consumption"]').text()).toContain('today-model')
    expect(getModels).toHaveBeenCalledTimes(2)
    expect(getDetails).toHaveBeenCalledTimes(1)
  })

  it('cancels and ignores a stale model response when changing users', async () => {
    const old = deferred<{ models: { model: string; actual_cost: number }[] }>()
    getDetails.mockResolvedValueOnce(response(1)).mockResolvedValueOnce(response(2))
    getModels.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ models: [{ model: 'current-user-model', actual_cost: 1 }] })
    const wrapper = open(); await flushPromises()
    const signal = getModels.mock.calls[0][1].signal as AbortSignal
    await wrapper.setProps({ user: user(2) }); await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(getModels.mock.calls[1][0].user_id).toBe(2)
    old.resolve({ models: [{ model: 'other-user-model', actual_cost: 99 }] }); await flushPromises()
    expect(wrapper.get('[data-test="usage-model-consumption"]').text()).toContain('current-user-model')
    expect(wrapper.get('[data-test="usage-model-consumption"]').text()).not.toContain('other-user-model')
  })

  it('keeps usage details available if the independent distribution request fails and can retry', async () => {
    getDetails.mockResolvedValue(response())
    getModels.mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce({ models: [{ model: 'recovered', actual_cost: 0.000736 }] })
    const wrapper = open(); await flushPromises()
    expect(wrapper.find('[data-test="usage-model-error"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="day-cost"]').text()).toBe('$4.00')
    await wrapper.get('[data-test="usage-model-error"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-test="usage-model-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="usage-model-row"]').text()).toContain('$0.000736')
  })

  it('cancels distribution loading when closed and clears it on reopening', async () => {
    const old = deferred<{ models: { model: string; actual_cost: number }[] }>()
    getDetails.mockResolvedValue(response())
    getModels.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ models: [] })
    const wrapper = open(); await flushPromises()
    const signal = getModels.mock.calls[0][1].signal as AbortSignal
    await wrapper.setProps({ show: false }); await flushPromises()
    expect(signal.aborted).toBe(true)
    old.resolve({ models: [{ model: 'stale', actual_cost: 50 }] }); await flushPromises()
    await wrapper.setProps({ show: true }); await flushPromises()
    expect(getModels).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('[data-test="usage-model-row"]')).toHaveLength(0)
    expect(wrapper.get('[data-test="usage-model-consumption"]').text()).not.toContain('NaN')
  })
})
