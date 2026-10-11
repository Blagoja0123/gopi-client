<script lang="ts">
  import type { Body } from '../lib/types'
  import CodeEditor from './CodeEditor.svelte'
  import KVTable from './KVTable.svelte'

  type Props = { body: Body | undefined; missing?: string[] }
  let { body = $bindable(), missing = [] }: Props = $props()

  const type = $derived(body?.type || 'none')
  const types = [
    ['none', 'None'], ['json', 'JSON'], ['text', 'Text'], ['xml', 'XML'],
    ['form', 'Form URL-encoded'], ['multipart', 'Multipart form'],
  ]

  function setType(t: string) {
    if (t === 'none') {
      body = undefined
      return
    }
    body = { ...(body ?? {}), type: t }
    if ((t === 'form' || t === 'multipart') && !body.form) body.form = []
  }

  function format() {
    if (!body?.content) return
    try {
      body.content = JSON.stringify(JSON.parse(body.content), null, 2)
    } catch {
      /* leave invalid JSON (may contain {{vars}}) alone */
    }
  }
</script>

<div class="body">
  <div class="bar row">
    {#each types as [v, label]}
      <label class="opt"><input type="radio" name="bt" checked={type === v} onchange={() => setType(v)} /> {label}</label>
    {/each}
    <span class="spacer"></span>
    {#if type === 'json'}<button class="btn sm ghost" onclick={format}>Format</button>{/if}
  </div>
  <div class="content">
    {#if body && (type === 'json' || type === 'text' || type === 'xml')}
      <CodeEditor bind:value={body.content} language={type === 'json' ? 'json' : type === 'xml' ? 'xml' : 'text'} placeholder={type === 'json' ? '{ "key": "{{value}}" }' : ''} />
    {:else if body && (type === 'form' || type === 'multipart')}
      <KVTable bind:rows={body.form} {missing} fileHint={type === 'multipart'} />
    {:else}
      <div class="empty">This request has no body.</div>
    {/if}
  </div>
</div>

<style>
  .body { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .bar { padding: 6px 10px; flex-wrap: wrap; gap: 12px; flex-shrink: 0; }
  .opt { display: flex; align-items: center; gap: 4px; cursor: pointer; }
  .content { flex: 1; min-height: 0; overflow: auto; border-top: 1px solid var(--border); }
</style>
