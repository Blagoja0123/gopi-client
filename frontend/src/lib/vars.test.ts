import { describe, expect, it } from 'vitest'
import { tokenize, varNames } from './vars'

describe('tokenize', () => {
  it('splits text and variables', () => {
    expect(tokenize('a {{b}} c')).toEqual([
      { text: 'a ' },
      { text: '{{b}}', name: 'b' },
      { text: ' c' },
    ])
  })
  it('allows whitespace and $ names', () => {
    expect(tokenize('{{ host }}/{{$uuid}}')).toEqual([
      { text: '{{ host }}', name: 'host' },
      { text: '/' },
      { text: '{{$uuid}}', name: '$uuid' },
    ])
  })
  it('treats unterminated braces as text', () => {
    expect(tokenize('{{oops')).toEqual([{ text: '{{oops' }])
  })
  it('handles empty input', () => {
    expect(tokenize('')).toEqual([])
  })
  it('round-trips the original text', () => {
    const s = 'x{{a}}{{b}}y{{ c }}'
    expect(tokenize(s).map((t) => t.text).join('')).toBe(s)
  })
})

describe('varNames', () => {
  it('lists unique names in order', () => {
    expect(varNames('{{a}} {{b}} {{a}}')).toEqual(['a', 'b'])
  })
})
