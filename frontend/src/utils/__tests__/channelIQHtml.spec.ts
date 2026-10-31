import { describe, expect, it } from 'vitest'
import { extractChannelIQAnimationDocument } from '../channelIQHtml'

describe('channel IQ animation extraction', () => {
  it('extracts fenced HTML and keeps declarative SVG animation content', () => {
    const html = extractChannelIQAnimationDocument(`说明文字
\`\`\`html
<!doctype html>
<html><head><style>.pelican { animation: fly 1s infinite; }</style></head>
<body><main><svg viewBox="0 0 10 10"><circle class="pelican" cx="5" cy="5" r="4"/><animate attributeName="opacity" values="1;.4;1" dur="1s" repeatCount="indefinite"/></svg></main></body></html>
\`\`\`
解释文字`)

    expect(html).toContain('<style>')
    expect(html).toContain('<svg')
    expect(html).toContain('<animate')
    expect(html).toContain('animation: fly')
    expect(html).toContain('width: 100% !important')
    expect(html).toContain('align-items: center !important')
  })

  it('removes scripts, event handlers, and non-image external resources', () => {
    const html = extractChannelIQAnimationDocument(`
      <svg viewBox="0 0 10 10" onclick="alert(1)">
        <script>alert(1)</script>
        <image href="https://example.com/tracker.gif" />
        <circle cx="5" cy="5" r="4" />
      </svg>
    `)

    expect(html).toContain('<svg')
    expect(html).toContain('<circle')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('onclick')
    expect(html).not.toContain('https://example.com')
  })

  it('does not turn a plain answer into an animation document', () => {
    expect(extractChannelIQAnimationDocument('2 + 2 = 4')).toBe('')
  })
})
