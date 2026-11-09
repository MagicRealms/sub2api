import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import TesterHistoryResetButton from '../TesterHistoryResetButton.vue'

const state = vi.hoisted(() => ({
  auth: { isTester: true }, reset: vi.fn(), success: vi.fn(), error: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state.auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess:state.success, showError:state.error }) }))
vi.mock('@/api/tester', () => ({ resetTesterHistory:state.reset }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t:(key:string) => key }) }))

function render() {
  return mount(TesterHistoryResetButton, { global: { stubs: {
    Icon:true,
    BaseDialog:{ props:['show'], template:'<div v-if="show"><slot/><slot name="footer"/></div>' },
  } } })
}
describe('tester history clear control', () => {
  beforeEach(() => { state.auth.isTester=true; vi.clearAllMocks() })
  it('requires confirmation and prevents duplicate clears while the request is pending', async () => {
    let finish!: (value:unknown) => void
    state.reset.mockImplementation(() => new Promise(resolve => { finish=resolve }))
    const w=render()
    await w.get('button').trigger('click')
    expect(w.text()).toContain('tester.clearHint')
    expect(state.reset).not.toHaveBeenCalled()
    const confirm=w.findAll('button').find(b=>b.text()==='tester.confirmClear')!
    await confirm.trigger('click')
    await confirm.trigger('click')
    expect(state.reset).toHaveBeenCalledTimes(1)
    expect(confirm.attributes('disabled')).toBeDefined()
    finish({ visible_after:'2026-11-09T10:00:00Z',billing_preserved:true })
    await flushPromises()
    expect(w.emitted('reset')).toEqual([['2026-11-09T10:00:00Z']])
    expect(state.success).toHaveBeenCalledWith('tester.cleared')
    w.unmount()
  })
  it('does not expose clearing to ordinary users or administrators', () => {
    state.auth.isTester=false
    const w=render()
    expect(w.find('button').exists()).toBe(false)
    expect(state.reset).not.toHaveBeenCalled()
    w.unmount()
  })
  it('keeps the dialog open and shows an error when clearing fails', async () => {
    state.reset.mockRejectedValue(new Error('failed'))
    const w=render()
    await w.get('button').trigger('click')
    await w.findAll('button').find(b=>b.text()==='tester.confirmClear')!.trigger('click')
    await flushPromises()
    expect(w.emitted('reset')).toBeUndefined()
    expect(w.text()).toContain('tester.clearHint')
    expect(state.error).toHaveBeenCalledWith('tester.clearFailed')
    w.unmount()
  })
})
