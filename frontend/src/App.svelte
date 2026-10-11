<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from './lib/state.svelte'
  import { on } from './lib/api'
  import Sidebar from './components/Sidebar.svelte'
  import TabBar from './components/TabBar.svelte'
  import RequestEditor from './components/RequestEditor.svelte'
  import FolderEditor from './components/FolderEditor.svelte'
  import WebhookPanel from './components/WebhookPanel.svelte'
  import EnvEditor from './components/EnvEditor.svelte'
  import ImportDialog from './components/ImportDialog.svelte'
  import Dialog from './components/Dialog.svelte'

  onMount(() => {
    app.init()
    return on('workspace:changed', () => app.onDiskChange())
  })

  function onKey(e: KeyboardEvent) {
    if (!(e.ctrlKey || e.metaKey)) return
    if (e.key === 's') {
      e.preventDefault()
      app.save()
    } else if (e.key === 'w') {
      e.preventDefault()
      if (app.activeId) app.closeTab(app.activeId)
    } else if (e.key === 'n') {
      e.preventDefault()
      app.newScratch('http')
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="layout">
  <Sidebar />
  <main>
    <TabBar />
    <div class="content">
      {#if app.active?.type === 'request'}
        {#key app.active.id}
          <RequestEditor id={app.active.id} />
        {/key}
      {:else if app.active?.type === 'folder'}
        <FolderEditor id={app.active.id} />
      {:else if app.active?.type === 'webhooks'}
        <WebhookPanel />
      {:else}
        <div class="welcome">
          <h2>API Client</h2>
          <p class="muted">REST · gRPC · WebSocket · Webhooks</p>
          <div class="row">
            <button class="btn primary" onclick={() => app.newScratch('http')}>Quick HTTP request</button>
            <button class="btn" onclick={() => app.newScratch('grpc')}>gRPC</button>
            <button class="btn" onclick={() => app.newScratch('ws')}>WebSocket</button>
            <button class="btn" onclick={() => app.openWebhooks()}>Webhooks</button>
          </div>
          <p class="muted keys"><kbd>Ctrl</kbd>+<kbd>N</kbd> quick request · <kbd>Ctrl</kbd>+<kbd>Enter</kbd> send · <kbd>Ctrl</kbd>+<kbd>S</kbd> save · <kbd>Ctrl</kbd>+<kbd>W</kbd> close tab</p>
        </div>
      {/if}
    </div>
  </main>
</div>

{#if app.envEditor}<EnvEditor />{/if}
{#if app.importOpen}<ImportDialog />{/if}
<Dialog />

<style>
  .layout { display: grid; grid-template-columns: 270px minmax(0, 1fr); height: 100vh; }
  main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
  .content { flex: 1; min-height: 0; }
  .welcome { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; }
  .welcome h2 { margin: 0; }
  .welcome p { margin: 0; }
  .keys { margin-top: 16px !important; font-size: 12px; }
  kbd { font-family: var(--mono); font-size: 11px; background: var(--bg-3); padding: 1px 4px; border-radius: 3px; }
</style>
