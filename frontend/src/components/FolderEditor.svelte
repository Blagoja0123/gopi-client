<script lang="ts">
  import { app, isDirty, type FolderTab } from '../lib/state.svelte'
  import KVTable from './KVTable.svelte'
  import AuthEditor from './AuthEditor.svelte'

  let { id }: { id: string } = $props()
  const tab = $derived(app.tabs.find((t) => t.id === id) as FolderTab)
</script>

<div class="folder">
  <div class="top row">
    <h3>Folder settings: <span class="mono">{tab.path || '/'}</span></h3>
    <span class="spacer"></span>
    <button class="btn sm" disabled={!isDirty(tab)} onclick={() => app.save(tab)}>Save</button>
  </div>
  <p class="muted">Saved to <code>{tab.path ? tab.path + '/' : ''}_folder.yaml</code>. Every request in this folder (and subfolders) inherits these headers, and uses this auth when its own auth is set to “Inherit”.</p>
  <div class="section-title">Default headers</div>
  <KVTable bind:rows={tab.folder.headers} />
  <div class="section-title">Auth</div>
  <AuthEditor bind:auth={tab.folder.auth} allowInherit={tab.path !== ''} />
</div>

<style>
  .folder { padding: 12px 16px; overflow: auto; height: 100%; }
  h3 { margin: 0; font-size: 15px; }
  p { font-size: 12px; }
  .section-title { margin-top: 12px; padding-left: 8px; }
  code { font-family: var(--mono); font-size: 11px; }
</style>
