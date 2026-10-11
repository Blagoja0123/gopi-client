import { describe, expect, it } from 'vitest'
import { splitUrl, syncUrlParams } from './url'

describe('splitUrl', () => {
  it('splits query params preserving order and decoding', () => {
    expect(splitUrl('https://x/a?b=2&a=hello%20world&flag')).toEqual({
      base: 'https://x/a',
      params: [
        { key: 'b', value: '2' },
        { key: 'a', value: 'hello world' },
        { key: 'flag', value: '' },
      ],
    })
  })
  it('keeps variables intact', () => {
    expect(splitUrl('{{base}}/x?id={{id}}')).toEqual({ base: '{{base}}/x', params: [{ key: 'id', value: '{{id}}' }] })
  })
  it('returns no params without a query', () => {
    expect(splitUrl('http://x')).toEqual({ base: 'http://x', params: [] })
  })
})

describe('syncUrlParams', () => {
  it('moves pasted query params into the params list', () => {
    const r = syncUrlParams('http://x?a=1', [{ key: 'z', value: '0' }])
    expect(r.url).toBe('http://x')
    expect(r.params).toEqual([
      { key: 'z', value: '0' },
      { key: 'a', value: '1' },
    ])
  })
  it('replaces an existing param with the same key', () => {
    const r = syncUrlParams('http://x?a=2', [{ key: 'a', value: '1', disabled: true }])
    expect(r.params).toEqual([{ key: 'a', value: '2' }])
  })
  it('is a no-op without a query', () => {
    const params = [{ key: 'a', value: '1' }]
    expect(syncUrlParams('http://x', params)).toEqual({ url: 'http://x', params })
  })
})
