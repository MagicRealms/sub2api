import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import VersionBadge from '../VersionBadge.vue'

const state = vi.hoisted(() => ({
  auth: { isAdmin: true },
  app: {
    versionLoading: false,
    currentVersion: '0.2.5-mr.123',
    latestVersion: '0.2.6',
    hasUpdate: true,
    buildType: 'source',
    versionWarning: '',
    releaseInfo: { html_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.6' },
    fetchVersion: vi.fn(),
    clearVersionCache: vi.fn()
  }
}))

vi.mock('@/stores', () => ({ useAuthStore: () => state.auth, useAppStore: () => state.app }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/system', () => ({
  performUpdate: vi.fn(), restartService: vi.fn(), getRollbackVersions: vi.fn(), rollback: vi.fn()
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }) }))

let unmount: (() => void) | undefined
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  state.auth.isAdmin = true
  state.app.hasUpdate = true
  state.app.versionWarning = ''
})
afterEach(() => {
  unmount?.()
  unmount = undefined
  vi.useRealTimers()
})

function renderBadge(version?: string) {
  const wrapper = mount(VersionBadge, { props: { version }, global: { stubs: { Icon: true } } })
  unmount = () => wrapper.unmount()
  return wrapper
}

describe('managed fork update notifications', () => {
  it('shows the upstream release and merge instructions without binary update or rollback actions', async () => {
    const wrapper = renderBadge()
    expect(wrapper.text()).toContain(`v${state.app.currentVersion}`)
    expect(wrapper.text()).not.toContain('做一个稳定的中转站')
    expect(wrapper.get('button').attributes('title')).toBe('version.upstreamUpdateAvailable')
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('version.forkUpdateHint')
    expect(wrapper.get('a').attributes('href')).toBe(state.app.releaseInfo.html_url)
    expect(wrapper.text()).not.toContain('version.updateNow')
    expect(wrapper.text()).not.toContain('version.rollback')
    await wrapper.get('button[title="version.refresh"]').trigger('click')
    expect(state.app.fetchVersion).toHaveBeenLastCalledWith(true)
  })

  it('continues checking while the admin page is open and stops when unmounted', () => {
    const wrapper = renderBadge()
    expect(state.app.fetchVersion).toHaveBeenCalledWith(false)
    vi.advanceTimersByTime(20 * 60 * 1000)
    expect(state.app.fetchVersion).toHaveBeenCalledTimes(21)
    wrapper.unmount()
    unmount = undefined
    vi.advanceTimersByTime(20 * 60 * 1000)
    expect(state.app.fetchVersion).toHaveBeenCalledTimes(21)
  })

  it('shows a failed check instead of reporting that the version is current', async () => {
    state.app.hasUpdate = false
    state.app.versionWarning = 'github unavailable'
    const wrapper = renderBadge()
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('version.checkFailed')
    expect(wrapper.text()).not.toContain('version.upToDate')
  })

  it.each([undefined, '0.2.17-mr.private-build'])('shows only the slogan and does not check releases for non-admins (version: %s)', (version) => {
    state.auth.isAdmin = false
    const wrapper = renderBadge(version)
    expect(wrapper.text()).toBe('做一个稳定的中转站')
    expect(wrapper.html()).not.toContain(state.app.currentVersion)
    if (version) expect(wrapper.html()).not.toContain(version)
    expect(wrapper.find('button').exists()).toBe(false)
    vi.advanceTimersByTime(20 * 60 * 1000)
    expect(state.app.fetchVersion).not.toHaveBeenCalled()
  })
})
