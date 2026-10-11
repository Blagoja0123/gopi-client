<script lang="ts">
  import type { RunResult } from '../lib/types'
  import { bodyLanguage, formatBytes, formatDuration, prettyBody, statusTone } from '../lib/format'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
  import { toast } from '../lib/dialog.svelte'
  import CodeEditor from './CodeEditor.svelte'

  type Props = { result: RunResult | null; error: string; running: boolean }
  let { result, error, running }: Props = $props()

  let view = $state<'body' | 'headers' | 'request'>('body')
  let raw = $state(false)

  const resp = $derived(result?.response)
  const shown = $derived(resp ? (raw || resp.bodyBase64 ? resp.body : prettyBody(resp.body, resp.contentType)) : '')
  const lang = $derived(resp && !raw ? bodyLanguage(resp.contentType) : 'text')

  async function copy() {
    if (!resp) return
    await ClipboardSetText(resp.body)
    toast('Body copied')
  }
</script>

<div class="resp">
  {#if running}
    <div class="empty">Sending…</div>
  {:else if error}
    <div class="err selectable">
      <div class="section-title">Request failed</div>
      <pre class="mono">{error}</pre>
    </div>
  {:else if resp}
    <div class="status row">
      <span class="code tone-{statusTone(resp.status)}">{resp.statusText}</span>
      <span class="pill">{formatDuration(resp.durationMs)}</span>
      <span class="pill">{formatBytes(resp.size)}{resp.truncated ? ' (truncated)' : ''}</span>
      <span class="spacer"></span>
      <button class="btn sm ghost" onclick={copy}>Copy</button>
    </div>
    {#if result?.missing?.length}
      <div class="warnbox">Unresolved variables were sent literally: {result.missing.join(', ')}</div>
    {/if}
    <div class="subtabs">
      <button class:on={view === 'body'} onclick={() => (view = 'body')}>Body</button>
      <button class:on={view === 'headers'} onclick={() => (view = 'headers')}>Headers<span class="count">{resp.headers?.length ?? 0}</span></button>
      <button class:on={view === 'request'} onclick={() => (view = 'request')}>Request</button>
      <span class="spacer"></span>
      {#if view === 'body' && !resp.bodyBase64}
        <label class="rawtoggle muted"><input type="checkbox" bind:checked={raw} /> Raw</label>
      {/if}
    </div>
    <div class="content">
      {#if view === 'body'}
        {#if resp.bodyBase64}
          <div class="empty">Binary response ({formatBytes(resp.size)}, {resp.contentType || 'unknown type'}).</div>
        {:else if resp.body === ''}
          <div class="empty">Empty body</div>
        {:else}
          <CodeEditor value={shown} language={lang} readonly />
        {/if}
      {:else if view === 'headers'}
        <table class="kv">
          <tbody>
            {#each resp.headers ?? [] as h}
              <tr><td class="k">{h.key}</td><td class="v">{h.value}</td></tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <table class="kv">
          <tbody>
            <tr><td class="k">URL</td><td class="v"><span class="method m-{resp.method}">{resp.method}</span> {resp.url}</td></tr>
            <tr><td class="k">Protocol</td><td class="v">{resp.proto}</td></tr>
            {#each resp.requestHeaders ?? [] as h}
              <tr><td class="k">{h.key}</td><td class="v">{h.value}</td></tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {:else}
    <div class="empty">
      Press <b>Send</b> or <kbd>Ctrl</kbd>+<kbd>Enter</kbd> to see the response.
    </div>
  {/if}
</div>

<style>
  .resp { display: flex; flex-direction: column; height: 100%; min-height: 0; gap: 0; }
  .status { padding: 8px 10px; flex-shrink: 0; }
  .code { font-weight: 700; font-family: var(--mono); }
  .warnbox { margin: 0 10px 6px; }
  .content { flex: 1; min-height: 0; overflow: auto; }
  .err { padding: 12px; }
  .err pre { white-space: pre-wrap; color: var(--error); }
  .rawtoggle { display: flex; align-items: center; gap: 4px; font-size: 12px; }
  .subtabs { align-items: center; }
  kbd { font-family: var(--mono); font-size: 11px; background: var(--bg-3); padding: 1px 4px; border-radius: 3px; }
</style>
