import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import type { ModelPlazaGroup, ModelPlazaResponse } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true }) }))

function group(id: number, overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id, name: `Group ${id}`, description: '', platform: 'openai', subscription_type: 'standard',
    rate_multiplier: 0.3, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1,
    is_exclusive: false, image_rate_independent: false, image_rate_multiplier: 1,
    video_rate_independent: false, video_rate_multiplier: 1, long_context_pricing_enabled: true,
    models: ['gpt-6.1-sol', 'gpt-5.6-sol'].map(name => ({ name, platform: 'openai', pricing: null, official_pricing: null })),
    ...overrides
  }
}

function render(groups: ModelPlazaGroup[], description = '') {
  return mount(ModelPlazaContent, {
    props: { response: { groups, description } satisfies ModelPlazaResponse, loading: false },
    global: {
      stubs: {
        BaseDialog: { props: ['show', 'title'], template: '<div v-if="show" role="dialog"><slot /></div>' },
        PlazaGroupSection: true
      }
    }
  })
}

describe('Model Plaza cards and filters', () => {
  it('shows cards, opens the selected group pricing, and uses a zero personal rate', async () => {
    const wrapper = render([group(1), group(2, { user_rate_multiplier: 0 })])
    const cards = wrapper.findAll('[data-test="plaza-group-card"]')
    expect(cards).toHaveLength(2)
    expect(cards[0].text()).toContain('Group 2')
    expect(cards[0].find('dd').text()).toContain('0')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.find('table').exists()).toBe(false)
    await cards[0].trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.findComponent(PlazaGroupSection).props('group').id).toBe(2)
  })

  it('filters the preview by model without truncating the pricing dialog catalog', async () => {
    const wrapper = render([group(1), group(2, { models: [{ name: 'gpt-image-2', platform: 'openai', pricing: null, official_pricing: null }] })])
    await wrapper.find('input').setValue('  GPT-6.1  ')
    expect(wrapper.findAll('[data-test="plaza-group-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-test="plaza-group-card"]').text()).not.toContain('gpt-5.6-sol')
    await wrapper.find('[data-test="plaza-group-card"]').trigger('click')
    expect(wrapper.findComponent(PlazaGroupSection).props('group').models).toHaveLength(2)
    await wrapper.find('input').setValue('unknown-model')
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
  })

  it('combines group and rate filters and resets a rate removed by refreshed data', async () => {
    const wrapper = render([group(1), group(2, { rate_multiplier: 0.25 })])
    await wrapper.findAll('select')[1].setValue('0.25')
    expect(wrapper.findAll('[data-test="plaza-group-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-test="plaza-group-card"]').text()).toContain('Group 2')
    await wrapper.findAll('select')[0].setValue('2')
    expect(wrapper.findAll('[data-test="plaza-group-card"]')).toHaveLength(1)
    await wrapper.setProps({ response: { groups: [group(2)], description: '' } })
    expect(wrapper.findAll('[data-test="plaza-group-card"]')).toHaveLength(1)
    expect((wrapper.findAll('select')[1].element as HTMLSelectElement).value).toBe('all')
  })

  it('sanitizes configured Markdown and counts models once across groups', () => {
    const wrapper = render([group(1), group(2)], '<img src="x" onerror="alert(1)"><script>alert(1)</script>')
    expect(wrapper.find('[data-test="plaza-summary"]').findAll('dd').map(dd => dd.text())).toEqual(['2', '2'])
    expect(wrapper.find('.plaza-description').html()).not.toContain('onerror')
    expect(wrapper.find('.plaza-description').find('script').exists()).toBe(false)
  })
})
