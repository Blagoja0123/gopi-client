<script lang="ts">
  import { tokenize } from '../lib/vars'

  type Props = {
    value: string | undefined
    placeholder?: string
    missing?: string[]
    type?: string
    class?: string
    oninput?: (v: string) => void
    onkeydown?: (e: KeyboardEvent) => void
    onpaste?: (e: ClipboardEvent) => void
  }
  let { value = $bindable(), placeholder = '', missing = [], type = 'text', class: cls = '', oninput, onkeydown, onpaste }: Props = $props()

  let overlay = $state<HTMLDivElement>()
  const tokens = $derived(tokenize(value ?? ''))
  const showOverlay = $derived(type !== 'password' && tokens.some((t) => t.name))

  function sync(e: Event) {
    if (overlay) overlay.scrollLeft = (e.target as HTMLInputElement).scrollLeft
  }
</script>

<div class="vi {cls}">
  {#if showOverlay}
    <div class="overlay" bind:this={overlay} aria-hidden="true">
      {#each tokens as t, i (i)}{#if t.name}<span class="var" class:missing={missing.includes(t.name)} title={missing.includes(t.name) ? `${t.name} is not defined` : t.name}>{t.text}</span>{:else}<span>{t.text}</span>{/if}{/each}
    </div>
  {/if}
  <input
    class="input"
    class:transparent={showOverlay}
    {type}
    {placeholder}
    spellcheck="false"
    autocomplete="off"
    bind:value
    oninput={(e) => { sync(e); oninput?.((e.target as HTMLInputElement).value) }}
    onscroll={sync}
    onkeyup={sync}
    {onkeydown}
    {onpaste}
  />
</div>

<style>
  .vi { position: relative; min-width: 0; display: flex; }
  .vi input { width: 100%; font-family: var(--mono); font-size: 12px; }
  .vi input.transparent { color: transparent; caret-color: var(--text); background: transparent; position: relative; z-index: 1; }
  .overlay {
    position: absolute; inset: 0; padding: 6px 9px; white-space: pre; overflow: hidden;
    font-family: var(--mono); font-size: 12px; pointer-events: none; color: var(--text);
    border: 1px solid transparent; line-height: normal;
    display: flex; align-items: center;
  }
  .var { color: var(--var-ok); background: var(--var-ok-bg); border-radius: 3px; }
  .var.missing { color: var(--var-missing); background: var(--var-missing-bg); }
</style>
