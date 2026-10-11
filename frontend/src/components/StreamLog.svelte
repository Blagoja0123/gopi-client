<script lang="ts">
  import type { StreamEvent } from '../lib/types'
  import { prettyBody } from '../lib/format'
  import { formatDuration } from '../lib/format'

  type Props = { events: StreamEvent[]; onclear?: () => void; status?: string }
  let { events, onclear, status = '' }: Props = $props()

  let filter = $state('')
  let box: HTMLDivElement
  const shown = $derived(filter ? events.filter((e) => (e.data ?? '').toLowerCase().includes(filter.toLowerCase())) : events)

  // Stick to the bottom as messages arrive.
  $effect(() => {
    shown.length
    if (box && box.scrollHeight - box.scrollTop - box.clientHeight < 80) {
      queueMicrotask(() => box && (box.scrollTop = box.scrollHeight))
    }
  })

  function time(t: string) {
    const d = new Date(t)
    return isNaN(d.getTime()) ? '' : d.toLocaleTimeString(undefined, { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
  }

  const arrows: Record<string, string> = { sent: '↑', message: '↓', open: '●', close: '○', end: '■', headers: '≡', error: '!' }
</script>

<div class="log">
  <div class="bar row">
    <span class="muted">{status}</span>
    <span class="spacer"></span>
    <input class="input filter" placeholder="Filter" bind:value={filter} />
    <button class="btn sm ghost" onclick={onclear}>Clear</button>
  </div>
  <div class="items" bind:this={box}>
    {#each shown as e, i (i)}
      <div class="ev ev-{e.type}">
        <div class="meta row">
          <span class="arrow">{arrows[e.type] ?? '·'}</span>
          <span class="type">{e.type}</span>
          {#if e.type === 'end'}
            <span class:tone-ok={e.code === 'OK'} class:tone-error={e.code !== 'OK'} class="mono">{e.code}</span>
            {#if e.durationMs}<span class="muted">{formatDuration(e.durationMs)}</span>{/if}
          {/if}
          {#if e.binary}<span class="pill">binary</span>{/if}
          <span class="spacer"></span>
          <span class="muted mono">{time(e.time)}</span>
        </div>
        {#if e.data}
          <pre class="mono selectable">{prettyBody(e.data, '')}</pre>
        {/if}
        {#if e.metadata?.length}
          <table class="kv">
            <tbody>
              {#each e.metadata as m}<tr><td class="k">{m.key}</td><td class="v">{m.value}</td></tr>{/each}
            </tbody>
          </table>
        {/if}
      </div>
    {:else}
      <div class="empty">No messages yet.</div>
    {/each}
  </div>
</div>

<style>
  .log { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .bar { padding: 6px 10px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
  .filter { width: 160px; padding: 3px 7px; }
  .items { flex: 1; overflow: auto; min-height: 0; }
  .ev { padding: 6px 10px; border-bottom: 1px solid var(--border); }
  .ev pre { margin: 4px 0 0 18px; white-space: pre-wrap; word-break: break-all; }
  .arrow { width: 12px; text-align: center; font-weight: 700; }
  .type { font-size: 11px; text-transform: uppercase; color: var(--text-2); font-weight: 600; }
  .ev-sent .arrow { color: var(--accent); }
  .ev-message .arrow { color: var(--ok); }
  .ev-close .arrow, .ev-error .arrow { color: var(--error); }
  .ev-sent { background: var(--bg-2); }
</style>
