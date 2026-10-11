<script lang="ts">
  import { closeDialog, dialog, toasts } from '../lib/dialog.svelte'

  let values = $state<string[]>([])
  let first = $state<HTMLElement | null>(null)

  $effect(() => {
    const d = dialog.current
    values = d ? d.fields.map((f) => f.value) : []
    if (d) queueMicrotask(() => first?.focus())
  })

  function submit(e?: Event) {
    e?.preventDefault()
    closeDialog($state.snapshot(values))
  }
</script>

{#if dialog.current}
  {@const d = dialog.current}
  <div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && closeDialog(null)}
    onkeydown={(e) => e.key === 'Escape' && closeDialog(null)}>
    <form class="box" onsubmit={submit}>
      <h3>{d.title}</h3>
      {#if d.message}<p>{d.message}</p>{/if}
      {#each d.fields as f, i}
        <label class="col">
          <span class="muted">{f.label}</span>
          {#if f.options}
            <select class="input" bind:value={values[i]}>
              {#each f.options as o}<option value={o.value}>{o.label}</option>{/each}
            </select>
          {:else if i === 0}
            <input class="input" bind:value={values[i]} bind:this={first} />
          {:else}
            <input class="input" bind:value={values[i]} />
          {/if}
        </label>
      {/each}
      <div class="row btns">
        <span class="spacer"></span>
        <button type="button" class="btn" onclick={() => closeDialog(null)}>Cancel</button>
        <button type="submit" class="btn primary" class:dangerbtn={d.danger}>{d.confirmLabel}</button>
      </div>
    </form>
  </div>
{/if}

<div class="toasts">
  {#each toasts.items as t (t.id)}
    <div class="toast {t.kind} selectable">{t.text}</div>
  {/each}
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: #0006; display: flex; align-items: flex-start; justify-content: center; padding-top: 18vh; z-index: 100; }
  .box { width: min(440px, 92vw); background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 16px; display: flex; flex-direction: column; gap: 10px; box-shadow: 0 20px 60px #0006; }
  h3 { margin: 0; font-size: 15px; }
  p { margin: 0; color: var(--text-2); }
  .btns { margin-top: 4px; }
  .dangerbtn { background: var(--error); border-color: var(--error); }
  .toasts { position: fixed; bottom: 16px; right: 16px; display: flex; flex-direction: column; gap: 8px; z-index: 200; max-width: 420px; }
  .toast { background: var(--bg); border: 1px solid var(--border); border-left: 3px solid var(--accent); padding: 8px 12px; border-radius: var(--radius); box-shadow: 0 6px 20px #0003; }
  .toast.error { border-left-color: var(--error); }
</style>
