import { describe, expect, it } from 'vitest'
import { extractChannelIQAnimationDocument, extractChannelIQHtmlPreviewDocument, getChannelIQAnimationAspectRatio } from '../channelIQHtml'

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
    expect(html).toContain('viewBox="0 0 10 10"')
    expect(html).toContain('cx="5"')
    expect(html).not.toContain('<main')
    expect(html).toContain('preserveAspectRatio="xMidYMid meet"')
    expect(html).toContain('animation: fly')
    expect(html).toContain('width: 100% !important')
    expect(html).toContain('position: relative !important')
    expect(html).toContain('background: transparent !important')
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

  it('uses the SVG viewBox ratio for the preview viewport', () => {
    expect(getChannelIQAnimationAspectRatio('<svg viewBox="0 0 1000 625"></svg>')).toBe(1.6)
    expect(getChannelIQAnimationAspectRatio('<svg viewBox="0 0 1600 900"></svg>')).toBeCloseTo(16 / 9)
    expect(getChannelIQAnimationAspectRatio('plain answer')).toBe(1.6)
  })
})

describe('channel IQ full HTML preview', () => {
  it('keeps the page layout, inline animation script, and local SVG references', () => {
    const result = extractChannelIQHtmlPreviewDocument(`
      <html><head><style>.scene { animation: ride 1s infinite }</style></head>
      <body><main class="scene"><h1>Pelican ride</h1>
      <svg viewBox="0 0 800 500"><defs><circle id="wheel" r="20"/></defs>
      <use href="#wheel" x="100"/><use xlink:href="#wheel" x="300"/>
      </svg></main><script>requestAnimationFrame(() => document.body.dataset.moving = 'yes')</script></body></html>
    `, 'abc123456789==', 'preview-token', false)

    expect(result).toContain('<main class="scene">')
    expect(result).toContain('Pelican ride')
    expect(result).toContain('href="#wheel"')
    expect(result).toContain('xlink:href="#wheel"')
    expect(result).toContain('position: relative !important')
    expect(result).toContain('background: transparent !important')
    expect(result).toContain('requestAnimationFrame(() => document.body.dataset.moving')
    expect(result).toContain('animation-play-state: paused')
    expect(result).toContain('function fitContent()')
    expect(result).toContain("scale(' + scale + ')")
    expect(result).toContain('script-src \'nonce-abc123456789==\'')
    expect(result).not.toContain('<iframe')
  })

  it('removes external resources and event handlers while keeping inline animation code', () => {
    const result = extractChannelIQHtmlPreviewDocument(`
      <html><body><svg onclick="alert(1)"><use href="https://example.com/wheel.svg#wheel"/>
      <image href="https://example.com/tracker.png"/><circle r="10"/></svg>
      <script src="https://example.com/evil.js"></script>
      <script>document.querySelector('circle').setAttribute('r', '12')</script>
      </body></html>
    `, 'abc123456789==', 'preview-token', true)

    expect(result).not.toContain('https://example.com')
    expect(result).not.toContain('onclick=')
    expect(result).toContain("setAttribute('r', '12')")
    expect(result).toContain("connect-src 'none'")
  })

  it('rejects plain text and disables scripts when the page nonce is unavailable', () => {
    expect(extractChannelIQHtmlPreviewDocument('No animation', '', 'token', false)).toBe('')
    const result = extractChannelIQHtmlPreviewDocument('<svg><circle r="2"/></svg><script>alert(1)</script>', '', 'token', false)
    expect(result).toContain("script-src 'none'")
    expect(result).not.toContain('<script')
  })
})
