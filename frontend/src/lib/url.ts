import type { KV } from './types'

function decode(s: string): string {
  try {
    return decodeURIComponent(s.replace(/\+/g, ' '))
  } catch {
    return s
  }
}

/** Split a URL into its base and query params without touching {{vars}}. */
export function splitUrl(url: string): { base: string; params: KV[] } {
  const q = url.indexOf('?')
  if (q < 0) return { base: url, params: [] }
  const params = url
    .slice(q + 1)
    .split('&')
    .filter((p) => p !== '')
    .map((pair) => {
      const eq = pair.indexOf('=')
      return eq < 0 ? { key: decode(pair), value: '' } : { key: decode(pair.slice(0, eq)), value: decode(pair.slice(eq + 1)) }
    })
  return { base: url.slice(0, q), params }
}

/** When a URL with a query string is typed/pasted, move the query into the
 *  params table (replacing params with the same key). */
export function syncUrlParams(url: string, params: KV[]): { url: string; params: KV[] } {
  const { base, params: fromUrl } = splitUrl(url)
  if (fromUrl.length === 0) return { url, params }
  const out = [...params]
  for (const p of fromUrl) {
    const i = out.findIndex((x) => x.key === p.key)
    if (i >= 0) out[i] = p
    else out.push(p)
  }
  return { url: base, params: out }
}
