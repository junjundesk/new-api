import DOMPurify, { type Config } from 'dompurify'

export type HtmlContentVariant = 'inline' | 'isolated'

const isolatedContentSandbox =
  'allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation'

const isolatedSanitizeOptions = {
  ADD_ATTR: [
    'allowfullscreen',
    'autoplay',
    'class',
    'controls',
    'default',
    'id',
    'kind',
    'label',
    'loading',
    'loop',
    'muted',
    'playsinline',
    'poster',
    'preload',
    'referrerpolicy',
    'rel',
    'srclang',
    'style',
    'target',
  ],
  ADD_TAGS: ['audio', 'iframe', 'picture', 'source', 'style', 'track', 'video'],
  FORBID_ATTR: ['srcdoc'],
  FORBID_TAGS: ['base', 'embed', 'link', 'meta', 'object', 'script'],
  FORCE_BODY: true,
} satisfies Config

function hardenIsolatedHtml(html: string): string {
  if (typeof document === 'undefined') {
    return html
  }

  const template = document.createElement('template')
  template.innerHTML = html

  template.content.querySelectorAll('a[target="_blank"]').forEach((link) => {
    const rel = new Set(
      link.getAttribute('rel')?.split(/\s+/).filter(Boolean) ?? []
    )

    rel.add('noopener')
    rel.add('noreferrer')
    link.setAttribute('rel', [...rel].join(' '))
  })

  template.content.querySelectorAll('iframe').forEach((frame) => {
    frame.removeAttribute('srcdoc')
    frame.setAttribute('sandbox', isolatedContentSandbox)
    frame.setAttribute('referrerpolicy', 'no-referrer')

    if (!frame.hasAttribute('loading')) {
      frame.setAttribute('loading', 'lazy')
    }
  })

  return template.innerHTML
}

export function sanitizeHtmlContent(
  content: string,
  variant: HtmlContentVariant = 'inline'
): string {
  if (variant === 'isolated') {
    const html = DOMPurify.sanitize(content, isolatedSanitizeOptions)

    return hardenIsolatedHtml(html)
  }

  return DOMPurify.sanitize(content)
}
