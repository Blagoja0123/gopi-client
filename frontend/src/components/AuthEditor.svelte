<script lang="ts">
  import type { Auth } from '../lib/types'
  import VarInput from './VarInput.svelte'

  type Props = { auth: Auth | undefined; missing?: string[]; allowInherit?: boolean }
  let { auth = $bindable(), missing = [], allowInherit = true }: Props = $props()

  const type = $derived(auth?.type || (allowInherit ? 'inherit' : 'none'))

  function setType(t: string) {
    auth = { ...(auth ?? {}), type: t }
  }
</script>

<div class="auth">
  <label class="row">
    <span class="lbl">Type</span>
    <select class="input" value={type} onchange={(e) => setType((e.target as HTMLSelectElement).value)}>
      {#if allowInherit}<option value="inherit">Inherit from folder</option>{/if}
      <option value="none">No auth</option>
      <option value="bearer">Bearer token</option>
      <option value="basic">Basic auth</option>
      <option value="apikey">API key</option>
    </select>
  </label>
  {#if auth && type === 'bearer'}
    <label class="row"><span class="lbl">Token</span><VarInput bind:value={auth.token} {missing} placeholder={'{{token}}'} class="grow" /></label>
  {:else if auth && type === 'basic'}
    <label class="row"><span class="lbl">Username</span><VarInput bind:value={auth.username} {missing} class="grow" /></label>
    <label class="row"><span class="lbl">Password</span><VarInput bind:value={auth.password} {missing} class="grow" /></label>
  {:else if auth && type === 'apikey'}
    <label class="row"><span class="lbl">Key</span><VarInput bind:value={auth.key} {missing} placeholder="X-API-Key" class="grow" /></label>
    <label class="row"><span class="lbl">Value</span><VarInput bind:value={auth.value} {missing} class="grow" /></label>
    <label class="row">
      <span class="lbl">Add to</span>
      <select class="input" value={auth.in || 'header'} onchange={(e) => auth && (auth.in = (e.target as HTMLSelectElement).value === 'query' ? 'query' : undefined)}>
        <option value="header">Header</option>
        <option value="query">Query params</option>
      </select>
    </label>
  {:else if type === 'inherit'}
    <p class="muted">Uses the auth configured on the nearest parent folder (folder settings).</p>
  {/if}
  <p class="muted tip">Use <code>{'{{variables}}'}</code> for credentials and mark them <b>secret</b> in an environment so they never get committed.</p>
</div>

<style>
  .auth { padding: 10px; display: flex; flex-direction: column; gap: 8px; max-width: 560px; }
  .lbl { width: 80px; color: var(--text-2); flex-shrink: 0; }
  .row :global(.grow) { flex: 1; }
  .tip { font-size: 12px; margin-top: 8px; }
</style>
