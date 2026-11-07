import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OrderedGroupSelector from '../OrderedGroupSelector.vue'

vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const options = [
  { value: 6, label: 'Enterprise', platform: 'openai' as const, rate: 0.32 },
  { value: 16, label: 'Plus', platform: 'openai' as const, rate: 0.13 },
  { value: 20, label: 'Claude Code', platform: 'anthropic' as const, rate: 1 }
]
const create = (ids: number[] = []) => mount(OrderedGroupSelector, {
  props: { modelValue: ids, options },
  global: { stubs: { GroupOptionItem: true } }
})

describe('ordered key group selection', () => {
  it('appends groups across providers in selection order', async () => {
    const wrapper = create([6])
    await wrapper.findAll('input[type="checkbox"]')[2]!.setValue(true)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[6, 20]])
    await wrapper.setProps({ modelValue: [6, 20] })
    await wrapper.findAll('input[type="checkbox"]')[1]!.setValue(true)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[6, 20, 16]])
  })

  it('reorders routes and shows their new priority numbers', async () => {
    const wrapper = create([6, 16])
    await wrapper.findAll('[aria-label="keys.routing.moveUp"]')[1]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[16, 6]])
    await wrapper.setProps({ modelValue: [16, 6] })
    expect(wrapper.findAll('[data-route-id]').map(row => row.attributes('data-route-id'))).toEqual(['16', '6'])
    expect(wrapper.find('[data-route-id="16"]').text()).toContain('1')
    expect(wrapper.text()).toContain('keys.routing.billingHint')
  })

  it('retains selected groups when switching provider filters and allows removal', async () => {
    const wrapper = create([6, 20])
    await wrapper.setProps({ options: options.filter(option => option.platform === 'openai'), allOptions: options })
    expect(wrapper.find('[data-route-id="20"]').text()).toContain('Claude Code')
    await wrapper.find('[data-route-id="6"] [aria-label="common.remove"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[20]])
  })
})
