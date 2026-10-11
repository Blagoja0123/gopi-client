import { describe, expect, it } from 'vitest'
import { formatBytes, formatDuration, prettyBody, statusTone, bodyLanguage } from './format'

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [512, '512 B'],
    [1536, '1.5 KB'],
    [5 * 1024 * 1024, '5.0 MB'],
  ])('%d -> %s', (n, want) => expect(formatBytes(n)).toBe(want))
})

describe('formatDuration', () => {
  it.each([
    [0.4, '<1 ms'],
    [12.345, '12 ms'],
    [1234, '1.23 s'],
  ])('%d -> %s', (n, want) => expect(formatDuration(n)).toBe(want))
})

describe('prettyBody', () => {
  it('indents JSON', () => {
    expect(prettyBody('{"a":1,"b":[1,2]}', 'application/json')).toBe('{\n  "a": 1,\n  "b": [\n    1,\n    2\n  ]\n}')
  })
  it('detects JSON without content type', () => {
    expect(prettyBody('[1]', '')).toBe('[\n  1\n]')
  })
  it('leaves invalid JSON untouched', () => {
    expect(prettyBody('{nope', 'application/json')).toBe('{nope')
  })
  it('leaves other content untouched', () => {
    expect(prettyBody('<a/>', 'text/xml')).toBe('<a/>')
  })
})

describe('statusTone', () => {
  it.each([
    [200, 'ok'],
    [302, 'info'],
    [404, 'warn'],
    [503, 'error'],
    [0, 'error'],
  ])('%d -> %s', (s, want) => expect(statusTone(s)).toBe(want))
})

describe('bodyLanguage', () => {
  it.each([
    ['application/json; charset=utf-8', 'json'],
    ['application/problem+json', 'json'],
    ['text/html', 'xml'],
    ['application/xml', 'xml'],
    ['text/plain', 'text'],
  ])('%s -> %s', (ct, want) => expect(bodyLanguage(ct)).toBe(want))
})
