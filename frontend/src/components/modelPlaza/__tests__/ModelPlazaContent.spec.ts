import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import type { ModelPlazaGroup, ModelPlazaResponse, PlazaModel } from '@/api/modelPlaza'
import type { UserSupportedModelPricing } from '@/api/channels'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }) }))

function model(name: string, pricing: Partial<UserSupportedModelPricing> = {}): PlazaModel {
  return {
    name, platform: 'openai', official_pricing: null,
    pricing: { billing_mode: 'token', input_price: 0.000002, output_price: 0.000008, cache_write_price: null, cache_read_price: 0.0000002, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [], ...pricing }
  }
}

function group(id: number, overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id, name: `Group ${id}`, description: '', platform: 'openai', subscription_type: 'standard',
    rate_multiplier: 0.3, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1,
    is_exclusive: false, image_rate_independent: false, image_rate_multiplier: 1,
    video_rate_independent: false, video_rate_multiplier: 1, long_context_pricing_enabled: true,
    models: ['gpt-6.1-sol', 'gpt-5.6-sol'].map(name => model(name)),
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
  it('shows each model once, switches groups and opens only that model with selected prices', async () => {
    const wrapper = render([group(1), group(2, { rate_multiplier: 0.25, models: [model('gpt-6.1-sol', { input_price: 0.000004 })] })])
    const cards = wrapper.findAll('[data-test="plaza-model-card"]')
    expect(cards).toHaveLength(2)
    const card = wrapper.find('[data-model="gpt-6.1-sol"]')
    expect(card.findAll('option')).toHaveLength(2)
    expect(card.find('[data-test="model-input-price"]').text()).toBe('$1.00')
    expect(card.find('[data-test="model-output-price"]').text()).toBe('$2.00')
    expect(card.find('[data-test="model-cache-price"]').text()).toBe('$0.05')
    await card.find('select').setValue('1')
    expect(card.find('[data-test="model-input-price"]').text()).toBe('$0.60')
    expect(card.find('[data-test="model-output-price"]').text()).toBe('$2.40')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.find('table').exists()).toBe(false)
    await card.find('[data-test="model-pricing-button"]').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.findComponent(PlazaGroupSection).props('group').id).toBe(1)
    expect(wrapper.findComponent(PlazaGroupSection).props('group').models.map((m: PlazaModel) => m.name)).toEqual(['gpt-6.1-sol'])
  })

  it('searches models and limits in-card choices to the groups returned by the server', async () => {
    const wrapper = render([group(1), group(2, { models: [{ name: 'gpt-image-2', platform: 'openai', pricing: null, official_pricing: null }] })])
    await wrapper.find('input').setValue('  GPT-6.1  ')
    expect(wrapper.findAll('[data-test="plaza-model-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-test="plaza-model-card"]').text()).not.toContain('gpt-5.6-sol')
    expect(wrapper.find('[data-test="model-group-select"]').findAll('option').map(o => o.attributes('value'))).toEqual(['1'])
    await wrapper.find('input').setValue('unknown-model')
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
  })

  it('combines filters, removes unavailable choices and resets rates after refresh', async () => {
    const wrapper = render([group(1), group(2, { rate_multiplier: 0.25 })])
    await wrapper.findAll('select')[1].setValue('0.25')
    expect(wrapper.findAll('[data-test="plaza-model-card"]')).toHaveLength(2)
    expect(wrapper.find('[data-test="model-group-select"]').findAll('option').map(o => o.attributes('value'))).toEqual(['2'])
    await wrapper.findAll('select')[0].setValue('2')
    await wrapper.setProps({ response: { groups: [group(2)], description: '' } })
    expect(wrapper.findAll('[data-test="plaza-model-card"]')).toHaveLength(2)
    expect((wrapper.findAll('select')[1].element as HTMLSelectElement).value).toBe('all')
  })

  it('honors zero personal rates and falls back safely when a selected group disappears', async () => {
    const wrapper = render([group(1), group(2, { user_rate_multiplier: 0 })])
    const card = wrapper.find('[data-model="gpt-6.1-sol"]')
    expect((card.find('select').element as HTMLSelectElement).value).toBe('2')
    expect(card.find('[data-test="model-input-price"]').text()).toBe('$0.00')
    await wrapper.setProps({ response: { groups: [group(1)], description: '' } })
    expect((card.find('select').element as HTMLSelectElement).value).toBe('1')
    expect(card.find('[data-test="model-input-price"]').text()).toBe('$0.60')
  })

  it('uses image and video independent rates, including a free media rate', async () => {
    const wrapper = render([group(1, { user_rate_multiplier: 0, image_rate_independent: true, image_rate_multiplier: 2, video_rate_independent: true, video_rate_multiplier: 0,
      models: [model('gpt-image-2', { billing_mode: 'image', per_request_price: 0.05 }), model('video-model', { billing_mode: 'video', per_request_price: 1 })] })])
    expect(wrapper.find('[data-model="gpt-image-2"] [data-test="model-request-price"]').text()).toContain('$0.10')
    expect(wrapper.find('[data-model="video-model"] [data-test="model-request-price"]').text()).toContain('$0.00')
  })

  it('resolves relative tier prices and discloses variable prices instead of presenting one final price', () => {
    const wrapper = render([group(1, { models: [model('gpt-6.1-sol', { intervals: [
      { min_tokens: 200000, max_tokens: null, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: null, input_multiplier: 4 },
      { min_tokens: 0, max_tokens: 200000, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: null, input_multiplier: 2, output_multiplier: 1.5 }
    ] })] })])
    expect(wrapper.find('[data-test="model-input-price"]').text()).toBe('$1.20')
    expect(wrapper.find('[data-test="model-output-price"]').text()).toBe('$3.60')
    expect(wrapper.find('[data-test="plaza-model-card"]').text()).toContain('modelPlaza.cards.variablePricing')
  })

  it('shows missing prices honestly and preserves separate providers with the same model name', () => {
    const wrapper = render([group(1, { models: [{ name: 'custom', platform: 'openai', pricing: null, official_pricing: null }, { name: 'custom', platform: 'gemini', pricing: null, official_pricing: null }] })])
    expect(wrapper.findAll('[data-test="plaza-model-card"]')).toHaveLength(2)
    expect(wrapper.find('[data-test="model-input-price"]').text()).toBe('-')
    expect(wrapper.text()).toContain('modelPlaza.detail.noPricing')
  })

  it('sanitizes configured Markdown and counts models once across groups', () => {
    const wrapper = render([group(1), group(2)], '<img src="x" onerror="alert(1)"><script>alert(1)</script>')
    expect(wrapper.find('[data-test="plaza-summary"]').findAll('dd').map(dd => dd.text())).toEqual(['2', '2'])
    expect(wrapper.find('.plaza-description').html()).not.toContain('onerror')
    expect(wrapper.find('.plaza-description').find('script').exists()).toBe(false)
  })
})
