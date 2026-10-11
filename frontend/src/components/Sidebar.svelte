<script lang="ts">
  import { app } from '../lib/state.svelte'
  import { filterTree } from '../lib/tree'
  import TreeItem from './TreeItem.svelte'

  let query = $state('')
  let expanded = $state<Record<string, boolean>>({})
  let wsMenu = $state(false)
  let newMenu = $state(false)
  const filtered = $derived(app.tree ? filterTree(app.tree, query) : null)
  const envs = $derived(app.state.environments ?? [])

  function scratchLabel(r: { name: string; url: string }) {
    if (r.name && r.name !== 'Untitled') return r.name
    return r.url || 'Untitled'
  }
</script>

<aside class="sidebar">
  <div class="ws-header">
    <button class="ws-name" onclick={() => (wsMenu = !wsMenu)} title={app.state.workspace?.root ?? ''}>
      <span class="grow">{app.state.workspace?.name ?? 'No workspace'}</span>
      <span class="muted">▾</span>
    </button>
    {#if wsMenu}
      <div class="dropdown" role="menu" tabindex="-1" onmouseleave={() => (wsMenu = false)}>
        <button onclick={() => { wsMenu = false; app.openWorkspaceDialog() }}>Open workspace…</button>
        <button onclick={() => { wsMenu = false; app.createWorkspaceDialog() }}>New workspace…</button>
        {#if app.state.workspace}
          <button onclick={() => { wsMenu = false; app.closeWorkspace() }}>Close workspace</button>
        {/if}
        {#if app.state.recent?.length}
          <div class="sep">Recent</div>
          {#each app.state.recent as r}
            <button class="mono small" onclick={() => { wsMenu = false; app.openWorkspace(r) }}>{r}</button>
          {/each}
        {/if}
      </div>
    {/if}
  </div>

  {#if app.state.workspace}
    <div class="env row">
      <select class="input grow" value={app.state.activeEnv} onchange={(e) => app.setEnv((e.target as HTMLSelectElement).value)}>
        <option value="">No environment</option>
        {#each envs as e}<option value={e}>{e}</option>{/each}
      </select>
      <button class="btn sm" onclick={() => (app.envEditor = true)} title="Manage environments and variables">Env</button>
    </div>
  {/if}

  <div class="actions row">
    <button class="btn sm primary grow" onclick={() => app.newScratch('http')} title="One-off request (Ctrl+N) — no collection needed">+ Quick request</button>
    <div class="newwrap">
      <button class="btn sm" onclick={() => (newMenu = !newMenu)}>New ▾</button>
      {#if newMenu}
        <div class="dropdown right" role="menu" tabindex="-1" onmouseleave={() => (newMenu = false)}>
          <div class="sep">Scratch (not saved to a collection)</div>
          <button onclick={() => { newMenu = false; app.newScratch('http') }}>HTTP request</button>
          <button onclick={() => { newMenu = false; app.newScratch('grpc') }}>gRPC call</button>
          <button onclick={() => { newMenu = false; app.newScratch('ws') }}>WebSocket</button>
          {#if app.state.workspace}
            <div class="sep">In workspace</div>
            <button onclick={() => { newMenu = false; app.newRequest('', 'http') }}>HTTP request</button>
            <button onclick={() => { newMenu = false; app.newRequest('', 'grpc') }}>gRPC call</button>
            <button onclick={() => { newMenu = false; app.newRequest('', 'ws') }}>WebSocket</button>
            <button onclick={() => { newMenu = false; app.newFolder('') }}>Folder</button>
          {/if}
        </div>
      {/if}
    </div>
  </div>

  <div class="scroll">
    {#if app.state.workspace}
      <input class="input search" placeholder="Filter requests" bind:value={query} />
      <div class="tree" role="tree">
        {#if filtered}
          <TreeItem node={filtered} depth={0} bind:expanded forceOpen={query.trim() !== ''} />
        {/if}
        {#if !app.tree?.children?.length}
          <div class="hint muted">
            No requests yet. Create one, or <button class="link" onclick={() => (app.importOpen = true)}>import an OpenAPI spec</button>.
          </div>
        {/if}
      </div>
    {:else}
      <div class="hint muted">
        Open a folder (e.g. a git repo) as a workspace to keep requests in version-controlled files, or just use quick requests.
        <div class="row" style="margin-top: 8px">
          <button class="btn sm" onclick={() => app.openWorkspaceDialog()}>Open…</button>
          <button class="btn sm" onclick={() => app.createWorkspaceDialog()}>New…</button>
        </div>
      </div>
    {/if}

    {#if app.scratch.length}
      <div class="section-title sect">Scratch</div>
      {#each app.scratch as s (s.id)}
        <div class="item" role="button" tabindex="0"
          class:active={app.active?.type === 'request' && app.active.scratchId === s.id}
          onclick={() => app.openScratch(s)} onkeydown={(e) => e.key === 'Enter' && app.openScratch(s)}>
          <span class="method m-{s.request.kind === 'http' ? s.request.method : s.request.kind}">{s.request.kind === 'http' ? s.request.method : s.request.kind === 'ws' ? 'WS' : 'gRPC'}</span>
          <span class="name">{scratchLabel(s.request)}</span>
          <button class="icon-btn x" title="Delete scratch request" onclick={(e) => { e.stopPropagation(); app.deleteScratch(s.id) }}>✕</button>
        </div>
      {/each}
    {/if}
  </div>

  <div class="footer col">
    <button class="btn ghost left" onclick={() => app.openWebhooks()}>⇄ Webhooks</button>
    <button class="btn ghost left" onclick={() => (app.importOpen = true)}>⤓ Import (OpenAPI / cURL)</button>
  </div>
</aside>

<style>
  .sidebar { display: flex; flex-direction: column; background: var(--bg-2); border-right: 1px solid var(--border); min-height: 0; height: 100%; }
  .ws-header { position: relative; padding: 8px; }
  .ws-name {
    display: flex; width: 100%; align-items: center; gap: 6px; background: none; border: none; font-weight: 600; font-size: 14px;
    cursor: pointer; padding: 4px 6px; border-radius: 4px; text-align: left;
  }
  .ws-name:hover { background: var(--bg-3); }
  .env, .actions { padding: 0 8px 8px; }
  .scroll { flex: 1; overflow: auto; min-height: 0; padding-bottom: 8px; }
  .search { margin: 0 8px 6px; width: calc(100% - 16px); padding: 4px 8px; }
  .hint { padding: 8px 14px; font-size: 12px; }
  .link { background: none; border: none; color: var(--accent); cursor: pointer; padding: 0; }
  .sect { padding: 12px 14px 4px; }
  .item { display: flex; align-items: center; gap: 6px; padding: 3px 6px 3px 8px; cursor: pointer; border-radius: 4px; margin: 0 4px; white-space: nowrap; }
  .item:hover { background: var(--bg-3); }
  .item.active { background: var(--bg-3); box-shadow: inset 2px 0 0 var(--accent); }
  .item .method { width: 34px; font-size: 10px; text-align: right; flex-shrink: 0; }
  .item .name { overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .item .x { display: none; }
  .item:hover .x { display: block; }
  .footer { border-top: 1px solid var(--border); padding: 6px; gap: 2px; }
  .left { text-align: left; }
  .newwrap { position: relative; }
  .dropdown {
    position: absolute; left: 8px; right: 8px; top: 100%; z-index: 20; background: var(--bg); border: 1px solid var(--border);
    border-radius: var(--radius); padding: 4px; display: flex; flex-direction: column; box-shadow: 0 6px 24px #0004;
  }
  .dropdown.right { left: auto; right: 0; width: 230px; }
  .dropdown button { background: none; border: none; text-align: left; padding: 6px 8px; border-radius: 4px; cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dropdown button:hover { background: var(--bg-3); }
  .sep { font-size: 11px; color: var(--text-2); padding: 6px 8px 2px; }
  .small { font-size: 11px; }
</style>
