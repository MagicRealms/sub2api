import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ApiKey, Group } from '@/types'
import MonitorKeyPickerDialog from '../MonitorKeyPickerDialog.vue'

vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('MonitorKeyPickerDialog ordered routes', () => {
  it('shows and selects a key with Claude in the second route and searches its name', async () => {
    const groups = [{ id: 1, name: 'GPT primary', platform: 'openai' }, { id: 2, name: 'Claude secondary', platform: 'anthropic' }] as Group[]
    const key = { id: 99, name: 'Mixed key', key: 'sk-test', group: groups[0], group_id: 1, group_ids: [1, 2] } as ApiKey
    const wrapper = mount(MonitorKeyPickerDialog, {
      props: { show: true, loading: false, provider: 'anthropic', keys: [key], groups },
      global: { stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' },
      } },
    })
    expect(wrapper.text()).toContain('Mixed key')
    expect(wrapper.text()).toContain('1.GPT primary')
    expect(wrapper.text()).toContain('2.Claude secondary')
    await wrapper.get('input').setValue('Claude secondary')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    await wrapper.get('tbody tr').trigger('click')
    expect(wrapper.emitted('pick')?.[0]).toEqual([key])
    await wrapper.setProps({ provider: 'gemini' })
    expect(wrapper.findAll('tbody tr')).toHaveLength(0)
    wrapper.unmount()
  })
})
