import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelIQPreview from '../ChannelIQPreview.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('ChannelIQPreview', () => {
  it('keeps the generated HTML interactive and has no playback control', () => {
    const wrapper = mount(ChannelIQPreview, { props: {
      responseText: '<html><body><button>Restart</button><svg viewBox="0 0 10 10"></svg></body></html>',
      title: 'Animation preview',
    } })

    expect(wrapper.get('iframe').classes()).not.toContain('pointer-events-none')
    expect(wrapper.findAll('button')).toHaveLength(0)
    expect(wrapper.get('iframe').attributes('sandbox')).toBe('allow-scripts')
  })
})
