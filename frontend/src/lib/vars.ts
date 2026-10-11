export type Token = { text: string; name?: string }

const VAR_RE = /\{\{\s*([A-Za-z0-9_.\-$]+)\s*\}\}/g

/** Split text into plain and {{variable}} segments (for highlighting). */
export function tokenize(s: string): Token[] {
  const out: Token[] = []
  let last = 0
  for (const m of s.matchAll(VAR_RE)) {
    const i = m.index ?? 0
    if (i > last) out.push({ text: s.slice(last, i) })
    out.push({ text: m[0], name: m[1] })
    last = i + m[0].length
  }
  if (last < s.length) out.push({ text: s.slice(last) })
  return out
}

export function varNames(s: string): string[] {
  return [...new Set(tokenize(s).flatMap((t) => (t.name ? [t.name] : [])))]
}
