import type { StreamEvent } from './types'

/** Insert e into log, ordered by seq within its session. Earlier sessions'
 *  events are never reordered. */
export function insertOrdered(log: StreamEvent[], e: StreamEvent): void {
  let i = log.length
  while (i > 0 && log[i - 1].session === e.session && (log[i - 1].seq ?? 0) > (e.seq ?? 0)) i--
  log.splice(i, 0, e)
}
