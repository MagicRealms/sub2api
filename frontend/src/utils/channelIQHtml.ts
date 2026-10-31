import DOMPurify from 'dompurify'

const channelIQSanitizeConfig = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  ADD_TAGS: ['animate', 'animateMotion', 'animateTransform', 'set'],
  FORBID_TAGS: ['base', 'embed', 'form', 'iframe', 'input', 'link', 'meta', 'object', 'script', 'textarea'],
  FORBID_ATTR: ['srcdoc'],
  // SVG animations may use fragment references such as href="#path".
  // External URLs and executable/data documents are unnecessary for a card
  // preview and could make a model response reach outside the site.
  ALLOWED_URI_REGEXP: /^(?:#|data:image\/(?:gif|png|jpeg|webp);base64,)/i,
}

function buildSafeStyleBlock(styles: Element[]): string {
  const css = styles
    .map(style => style.textContent || '')
    .join('\n')
    // The preview is declarative SVG/CSS. Drop imports and external URL
    // references so a model response cannot make the browser fetch arbitrary
    // resources from inside the card.
    .replace(/@import[\s\S]*?;\s*/gi, '')
    .replace(/url\(\s*(['"]?)(?!#|data:image\/(?:gif|png|jpeg|webp);base64,)[^)]*\1\s*\)/gi, 'none')
    .replace(/<\/?style/gi, '')
    .trim()
  return css ? `<style>${css}</style>` : ''
}

function unwrapHtmlSource(value: string): string {
  const fenced = [...value.matchAll(/```(?:html|xml|svg)?\s*([\s\S]*?)```/gi)]
    .map(match => match[1].trim())
    .find(candidate => /<svg\b/i.test(candidate))
  let source = (fenced || value).trim()
  const start = source.search(/<!doctype\s+html\b|<html\b|<svg\b/i)
  if (start >= 0) source = source.slice(start)

  const lower = source.toLowerCase()
  if (/<html\b/i.test(source)) {
    const end = lower.lastIndexOf('</html>')
    if (end >= 0) source = source.slice(0, end + '</html>'.length)
  } else {
    const end = lower.lastIndexOf('</svg>')
    if (end >= 0) source = source.slice(0, end + '</svg>'.length)
  }
  return source
}

/**
 * Extract a model-generated HTML document containing SVG and return a safe
 * iframe document. The iframe keeps model CSS scoped to the animation card;
 * DOMPurify removes scripts, event handlers, and unsafe embedded resources.
 */
export function extractChannelIQAnimationDocument(value: string): string {
  const raw = value.trim()
  if (!raw || !/<svg\b/i.test(raw)) return ''

  const source = unwrapHtmlSource(raw)
  if (!/<svg\b/i.test(source)) return ''

  const parsed = new DOMParser().parseFromString(source, 'text/html')
  if (!parsed.querySelector('svg')) return ''

  const styles = Array.from(parsed.querySelectorAll('style'))
  const body = parsed.body.cloneNode(true) as HTMLElement
  body.querySelectorAll('style').forEach(style => style.remove())
  const bodyMarkup = body.innerHTML || parsed.querySelector('svg')?.outerHTML || ''
  const sanitizedBody = DOMPurify.sanitize(bodyMarkup, channelIQSanitizeConfig)
  // DOMPurify intentionally removes HTML <style> nodes. The style text has
  // already been reduced to declarative, local CSS above, so prepend that
  // isolated block after sanitizing the markup.
  const sanitized = `${buildSafeStyleBlock(styles)}\n${sanitizedBody}`
  return /<svg\b/i.test(sanitized) ? sanitized : ''
}
