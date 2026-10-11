import { describe, expect, it } from 'vitest'
import { insertOrdered } from './stream'
import type { StreamEvent } from './types'

const ev = (session: string, seq: number, type = 'message'): StreamEvent => ({ session, seq, type, time: '' })

describe('insertOrdered', () => {
  it('restores order when a late event arrives after end', () => {
    const log: StreamEvent[] = []
    for (const e of [ev('a', 1, 'sent'), ev('a', 2, 'headers'), ev('a', 4, 'end'), ev('a', 3, 'message')]) insertOrdered(log, e)
    expect(log.map((e) => e.type)).toEqual(['sent', 'headers', 'message', 'end'])
  })
  it('never moves events before an earlier session', () => {
    const log: StreamEvent[] = [ev('a', 5)]
    insertOrdered(log, ev('b', 1))
    expect(log.map((e) => e.session)).toEqual(['a', 'b'])
  })
})
