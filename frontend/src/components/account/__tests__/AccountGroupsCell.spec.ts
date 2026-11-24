import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi } from 'vitest'
import type { Group } from '@/types'
import AccountGroupsCell from '../AccountGroupsCell.vue'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null })
}))

const groups = [
  { id: 31, name: '烈阳订阅', platform: 'composite', subscription_type: 'subscription' },
  { id: 6, name: 'GPT 企业级渠道', platform: 'openai', subscription_type: 'standard' },
  { id: 32, name: 'GPT 备线', platform: 'openai', subscription_type: 'standard' }
] as Group[]

function renderGroups(maxDisplay = 4, fallbackGroupIds = [6, 32]) {
  return mount(AccountGroupsCell, {
    props: { groups, fallbackGroupIds, maxDisplay },
    global: {
      plugins: [createI18n({
        legacy: false,
        locale: 'zh',
        messages: { zh: { admin: { accounts: {
          groupCountTotal: '共 {count} 个分组',
          finalFallback: {
            badge: '保底',
            badgeHint: '仅作最终保底',
            normalGroups: '正常分组',
            groups: '仅作保底的分组',
            membershipSummary: '正常分组 {normal} 个 · 保底分组 {fallback} 个'
          }
        } } } }
      })],
      stubs: {
        PlatformIcon: true,
        FloatingPanel: { props: ['open'], template: '<div v-if="open" data-popover><slot /></div>' }
      }
    }
  })
}

describe('account group membership roles', () => {
  it('marks fallback memberships without changing a normal subscription group', async () => {
    const wrapper = renderGroups()
    expect(wrapper.text()).toContain('烈阳订阅')
    const marked = wrapper.findAll('[data-fallback-badge]')
    expect(marked).toHaveLength(2)
    expect(marked.map(badge => badge.element.parentElement?.textContent)).toEqual([
      expect.stringContaining('GPT 企业级渠道'), expect.stringContaining('GPT 备线')
    ])
    expect(marked.every(badge => badge.attributes('class')?.includes('border-dashed'))).toBe(true)
    await wrapper.setProps({ fallbackGroupIds: [] })
    expect(wrapper.findAll('[data-fallback-badge]')).toHaveLength(0)
    expect(wrapper.text()).toContain('GPT 企业级渠道')
  })

  it('keeps fallback roles clear when memberships are hidden behind the overflow button', async () => {
    const wrapper = renderGroups(2)
    expect(wrapper.findAll('[data-fallback-badge]')).toHaveLength(0)
    const more = wrapper.find('button')
    expect(more.text()).toBe('+2')
    expect(more.attributes('title')).toBe('正常分组 1 个 · 保底分组 2 个')
    await more.trigger('click')
    const panel = wrapper.find('[data-popover]')
    expect(panel.text()).toContain('正常分组 · 1')
    expect(panel.text()).toContain('仅作保底的分组 · 2')
    expect(panel.findAll('[data-fallback-badge]')).toHaveLength(2)
  })
})
