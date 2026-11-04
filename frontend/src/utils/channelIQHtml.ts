import DOMPurify from 'dompurify'

const channelIQSanitizeConfig = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  ADD_TAGS: ['animate', 'animateMotion', 'animateTransform', 'set', 'use'],
  ADD_ATTR: [
    'accumulate', 'additive', 'alignment-baseline', 'attributeName', 'attributeType', 'begin', 'calcMode',
    'cx', 'cy', 'd', 'dur', 'end', 'fill', 'fill-opacity', 'filter', 'focusable', 'from', 'gradientTransform',
    'gradientUnits', 'height', 'href', 'keyPoints', 'keySplines', 'keyTimes', 'offset', 'opacity', 'path',
    'pathLength', 'points', 'preserveAspectRatio', 'repeatCount', 'role', 'r', 'rotate', 'rx', 'ry', 'stdDeviation',
    'stop-color', 'stop-opacity', 'stroke', 'stroke-linecap', 'stroke-linejoin', 'stroke-miterlimit', 'stroke-opacity',
    'stroke-width', 'to', 'transform', 'transform-origin', 'values', 'viewBox', 'version', 'width', 'x', 'x1', 'x2',
    'y', 'y1', 'y2', 'xmlns', 'xmlns:xlink', 'xlink:href',
  ],
  FORBID_TAGS: ['base', 'embed', 'form', 'iframe', 'input', 'link', 'meta', 'object', 'script', 'textarea'],
  FORBID_ATTR: ['srcdoc'],
}

// Model-generated documents often use a desktop-sized SVG (for example
// 800x500). Keep the complete scene inside the card viewport and let the
// browser scale it down or up to the available space.
const channelIQPreviewStyle = `<style>
html, body {
  width: 100% !important;
  height: 100% !important;
  min-width: 0 !important;
  min-height: 0 !important;
  margin: 0 !important;
  padding: 0 !important;
  overflow: hidden !important;
  background: transparent !important;
}
body {
  position: relative !important;
  display: block !important;
  box-sizing: border-box !important;
}
body > * {
  max-width: 100% !important;
  max-height: none !important;
  min-width: 0 !important;
  min-height: 0 !important;
  box-sizing: border-box !important;
}
body > :first-child {
  position: absolute !important;
  left: 50% !important;
  top: 50% !important;
  margin: 0 !important;
  transform-origin: center center !important;
}
body > svg {
  width: 100% !important;
  height: 100% !important;
}
body svg {
  max-width: 100% !important;
  max-height: 100% !important;
}
</style>`

const defaultChannelIQAspectRatio = 16 / 10

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

function sanitizeSvgResources(markup: string): string {
  const parsed = new DOMParser().parseFromString(markup, 'image/svg+xml')
  const root = parsed.documentElement
  if (!root || root.tagName.toLowerCase() !== 'svg') return ''

  const isSafeResource = (value: string): boolean => /^(?:#|data:image\/(?:gif|png|jpeg|webp);base64,)/i.test(value.trim())
  const stripExternalUrls = (value: string): string => value
    .replace(/url\(\s*(['"]?)(?!#|data:image\/(?:gif|png|jpeg|webp);base64,)[^)]*\1\s*\)/gi, 'none')
    .replace(/@import[\s\S]*?;\s*/gi, '')

  root.querySelectorAll('*').forEach(element => {
    for (const name of ['href', 'xlink:href', 'src']) {
      const value = element.getAttribute(name)
      if (value && !isSafeResource(value)) element.removeAttribute(name)
    }
    const style = element.getAttribute('style')
    if (style) element.setAttribute('style', stripExternalUrls(style))
    for (const attribute of Array.from(element.attributes)) {
      if (attribute.name !== 'style' && /url\(/i.test(attribute.value)) {
        element.setAttribute(attribute.name, stripExternalUrls(attribute.value))
      }
    }
  })
  return root.outerHTML
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
 * Return the SVG canvas ratio so the iframe can use the same shape as the
 * generated scene. This avoids letterboxing when different probes return
 * 16:9, 16:10, or another valid viewBox.
 */
export function getChannelIQAnimationAspectRatio(value: string): number {
  const source = unwrapHtmlSource(value.trim())
  const svgTag = source.match(/<svg\b[^>]*>/i)?.[0] || ''
  const viewBox = svgTag.match(/\bviewBox\s*=\s*["']\s*([+-]?[\d.]+)[\s,]+([+-]?[\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)\s*["']/i)
  const width = Number(viewBox?.[3])
  const height = Number(viewBox?.[4])
  if (Number.isFinite(width) && Number.isFinite(height) && width > 0 && height > 0) {
    return Math.min(2.4, Math.max(0.75, width / height))
  }
  return defaultChannelIQAspectRatio
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
  const svg = parsed.querySelector('svg')
  if (!svg) return ''
  // The surrounding HTML often contains a fixed desktop-sized .scene wrapper.
  // Render the SVG canvas itself so that wrapper dimensions cannot crop it.
  svg.setAttribute('preserveAspectRatio', 'xMidYMid meet')
  const bodyMarkup = svg.outerHTML
  const sanitizedBody = sanitizeSvgResources(DOMPurify.sanitize(bodyMarkup, channelIQSanitizeConfig))
  // DOMPurify intentionally removes HTML <style> nodes. The style text has
  // already been reduced to declarative, local CSS above, so prepend that
  // isolated block after sanitizing the markup.
  const sanitized = `${buildSafeStyleBlock(styles)}\n${channelIQPreviewStyle}\n${sanitizedBody}`
  return /<svg\b/i.test(sanitized) ? sanitized : ''
}

const channelIQFrameStyle = `<style>
html, body { width: 100%; min-height: 100%; margin: 0; overflow: hidden; }
html.channel-iq-paused *, html.channel-iq-paused *::before, html.channel-iq-paused *::after {
  animation-play-state: paused !important;
}
</style>`

function stripExternalResources(markup: string): string {
  const doc = new DOMParser().parseFromString(markup, 'text/html')
  const safeURL = (value: string): boolean => /^(?:#|data:image\/(?:gif|png|jpeg|webp);base64,)/i.test(value.trim())
  doc.querySelectorAll('*').forEach(element => {
    for (const name of ['href', 'xlink:href', 'src', 'srcset', 'action', 'formaction']) {
      const value = element.getAttribute(name)
      if (value && !safeURL(value)) element.removeAttribute(name)
    }
    const style = element.getAttribute('style')
    if (style) {
      element.setAttribute('style', style
        .replace(/@import[\s\S]*?;\s*/gi, '')
        .replace(/url\(\s*(['"]?)(?!#|data:image\/(?:gif|png|jpeg|webp);base64,)[^)]*\1\s*\)/gi, 'none'))
    }
  })
  return doc.body.innerHTML
}

function channelIQControlScript(token: string, autoplay: boolean): string {
  return `(() => {
  const token = ${JSON.stringify(token)};
  let playing = ${autoplay ? 'true' : 'false'};
  let pausedAt = playing ? 0 : performance.now();
  let pausedDuration = 0;
  let nextID = 1;
  const callbacks = new Map();
  const nativeRAF = window.requestAnimationFrame.bind(window);
  const nativeCancel = window.cancelAnimationFrame.bind(window);
  const content = () => Array.from(document.body.children)
    .find(element => !['SCRIPT', 'STYLE'].includes(element.tagName)) || null;

  function fitContent() {
    const root = content();
    if (!root) return;
    root.style.position = 'absolute';
    root.style.left = '50%';
    root.style.top = '50%';
    root.style.margin = '0';
    root.style.transform = 'translate(-50%, -50%)';
    const rect = root.getBoundingClientRect();
    const width = Math.max(rect.width, root.scrollWidth || 0, 1);
    const height = Math.max(rect.height, root.scrollHeight || 0, 1);
    const availableWidth = Math.max(window.innerWidth, 1);
    const availableHeight = Math.max(window.innerHeight, 1);
    const scale = Math.min(1, availableWidth / width, availableHeight / height);
    root.style.transform = 'translate(-50%, -50%) scale(' + scale + ')';
  }

  function reportReady() {
    fitContent();
    window.parent.postMessage({
      kind: 'channel-iq-ready', token,
      width: Math.max(document.body.scrollWidth, document.documentElement.scrollWidth),
      height: Math.max(document.body.scrollHeight, document.documentElement.scrollHeight),
    }, '*');
  }

  function schedule(id) {
    const pending = callbacks.get(id);
    if (!pending || !playing) return;
    pending.nativeID = nativeRAF(timestamp => {
      callbacks.delete(id);
      pending.callback(timestamp - pausedDuration);
    });
  }
  window.requestAnimationFrame = callback => {
    const id = nextID++;
    callbacks.set(id, { callback, nativeID: null });
    schedule(id);
    return id;
  };
  window.cancelAnimationFrame = id => {
    const pending = callbacks.get(id);
    if (pending && pending.nativeID !== null) nativeCancel(pending.nativeID);
    callbacks.delete(id);
  };

  function syncAnimations() {
    document.documentElement.classList.toggle('channel-iq-paused', !playing);
    document.querySelectorAll('svg').forEach(svg => {
      if (playing) svg.unpauseAnimations?.();
      else svg.pauseAnimations?.();
    });
  }
  function setPlaying(next) {
    if (next === playing) return;
    if (next) {
      pausedDuration += performance.now() - pausedAt;
      playing = true;
      syncAnimations();
      for (const id of callbacks.keys()) schedule(id);
    } else {
      playing = false;
      pausedAt = performance.now();
      for (const pending of callbacks.values()) {
        if (pending.nativeID !== null) nativeCancel(pending.nativeID);
        pending.nativeID = null;
      }
      syncAnimations();
    }
  }

  window.addEventListener('message', event => {
    if (event.source !== window.parent || event.data?.kind !== 'channel-iq-control' || event.data.token !== token) return;
    setPlaying(event.data.playing === true);
  });
  window.addEventListener('resize', fitContent, { passive: true });
  document.addEventListener('DOMContentLoaded', () => {
    syncAnimations();
    reportReady();
    setTimeout(fitContent, 0);
    setTimeout(fitContent, 120);
  });
})();`
}

/** Render model HTML in an opaque-origin iframe with only local, inline assets. */
export function extractChannelIQHtmlPreviewDocument(value: string, nonce: string, token: string, autoplay: boolean): string {
  const source = unwrapHtmlSource(value.trim())
  if (!/<svg\b/i.test(source)) return ''
  const parsed = new DOMParser().parseFromString(source, 'text/html')
  if (!parsed.querySelector('svg')) return ''

  const inlineScripts = Array.from(parsed.querySelectorAll<HTMLScriptElement>('script:not([src])'))
    .filter(script => !script.type || script.type === 'text/javascript')
    .map(script => script.textContent || '')
    .filter(Boolean)
  const styles = buildSafeStyleBlock(Array.from(parsed.querySelectorAll('style')))
  const body = stripExternalResources(DOMPurify.sanitize(parsed.body.innerHTML, channelIQSanitizeConfig))
  if (!/<svg\b/i.test(body)) return ''

  const safeNonce = /^[A-Za-z0-9+/=-]{8,128}$/.test(nonce) ? nonce : ''
  const scriptPolicy = safeNonce ? `'nonce-${safeNonce}'` : "'none'"
  const csp = `default-src 'none'; script-src ${scriptPolicy}; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; form-action 'none'; object-src 'none'; base-uri 'none'; navigate-to 'none'`
  const scripts = safeNonce
    ? [channelIQControlScript(token, autoplay), ...inlineScripts]
      .map(script => `<script nonce="${safeNonce}">${script.replace(/<\/script/gi, '<\\/script')}</script>`).join('\n')
    : ''
  return `<!doctype html><html${autoplay ? '' : ' class="channel-iq-paused"'}><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${csp}"><meta name="referrer" content="no-referrer">${styles}${channelIQPreviewStyle}${channelIQFrameStyle}</head><body>${body}${scripts}</body></html>`
}
