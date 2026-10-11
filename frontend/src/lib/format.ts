export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

export function formatDuration(ms: number): string {
  if (ms < 1) return '<1 ms'
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

export function bodyLanguage(contentType: string): 'json' | 'xml' | 'text' {
  const ct = contentType.toLowerCase()
  if (ct.includes('json')) return 'json'
  if (ct.includes('xml') || ct.includes('html')) return 'xml'
  return 'text'
}

/** Pretty-print JSON bodies; anything else is returned unchanged. */
export function prettyBody(body: string, contentType: string): string {
  const looksJson = /^\s*[[{]/.test(body)
  if (bodyLanguage(contentType) !== 'json' && !(contentType === '' && looksJson)) return body
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

export type Tone = 'ok' | 'info' | 'warn' | 'error'

export function statusTone(status: number): Tone {
  if (status >= 200 && status < 300) return 'ok'
  if (status >= 300 && status < 400) return 'info'
  if (status >= 400 && status < 500) return 'warn'
  return 'error'
}
