import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import StudioReply from './StudioReply.vue'

describe('studio reply rendering', () => {
  it('renders readable Markdown and keeps code examples as text', () => {
    const wrapper = mount(StudioReply, { props: { content: '**Hello**\n\n```html\n<script>alert(1)</script>\n```\n\n[Docs](https://example.com)' } })
    expect(wrapper.get('strong').text()).toBe('Hello')
    expect(wrapper.get('pre code').text()).toContain('<script>alert(1)</script>')
    expect(wrapper.get('a').attributes()).toMatchObject({ target: '_blank', rel: 'noopener noreferrer', href: 'https://example.com' })
    expect(wrapper.find('script').exists()).toBe(false)
  })
  it('removes executable markup, external images and unsafe links from model output', () => {
    const wrapper = mount(StudioReply, { props: { content: '<img src="https://example.com/track" onerror="alert(1)"><svg onload="alert(1)"></svg><iframe src="https://example.com"></iframe><a href="javascript:alert(1)">unsafe</a>' } })
    expect(wrapper.find('img, svg, iframe, script').exists()).toBe(false)
    expect(wrapper.find('a').attributes('href')).toBeUndefined()
    expect(wrapper.html()).not.toMatch(/onerror|onload|javascript:/)
  })
})
