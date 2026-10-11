<script lang="ts">
  import type { KV } from '../lib/types'
  import VarInput from './VarInput.svelte'

  type Props = {
    rows: KV[] | undefined
    missing?: string[]
    keyPlaceholder?: string
    valuePlaceholder?: string
    fileHint?: boolean
  }
  let { rows = $bindable(), missing = [], keyPlaceholder = 'Key', valuePlaceholder = 'Value', fileHint = false }: Props = $props()

  let draftKey = $state('')
  let draftValue = $state('')

  function commitDraft() {
    if (draftKey === '' && draftValue === '') return
    rows = [...(rows ?? []), { key: draftKey, value: draftValue }]
    draftKey = ''
    draftValue = ''
  }
</script>

<div class="kv">
  {#each rows ?? [] as row, i (i)}
    <div class="r" class:off={row.disabled}>
      <input type="checkbox" checked={!row.disabled} onchange={(e) => (row.disabled = !(e.target as HTMLInputElement).checked || undefined)} title="Enabled" />
      <VarInput bind:value={row.key} placeholder={keyPlaceholder} {missing} class="grow" />
      <VarInput bind:value={row.value} placeholder={valuePlaceholder} {missing} class="grow" />
      <button class="icon-btn" title="Remove" onclick={() => rows?.splice(i, 1)}>✕</button>
    </div>
  {/each}
  <div class="r draft">
    <span class="cb"></span>
    <input class="input mono grow" placeholder={keyPlaceholder} bind:value={draftKey} onblur={commitDraft} onkeydown={(e) => e.key === 'Enter' && commitDraft()} />
    <input class="input mono grow" placeholder={valuePlaceholder} bind:value={draftValue} onblur={commitDraft} onkeydown={(e) => e.key === 'Enter' && commitDraft()} />
    <span class="x"></span>
  </div>
  {#if fileHint}
    <div class="muted hint">Tip: a value of <code>@/path/to/file</code> uploads that file.</div>
  {/if}
</div>

<style>
  .kv { display: flex; flex-direction: column; gap: 4px; padding: 8px; }
  .r { display: flex; align-items: center; gap: 6px; }
  .r.off :global(input:not([type='checkbox'])) { opacity: 0.5; }
  .r :global(.grow) { flex: 1; min-width: 0; }
  .cb { width: 13px; flex-shrink: 0; margin: 0 3px; }
  .x { width: 22px; flex-shrink: 0; }
  .draft input { opacity: 0.8; }
  .hint { font-size: 11px; padding: 2px 24px; }
</style>
