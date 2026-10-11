<script lang="ts">
  import TreeItem from './TreeItem.svelte'
  import { app } from '../lib/state.svelte'
  import { api, errMsg } from '../lib/api'
  import { confirmAction, promptText, toast } from '../lib/dialog.svelte'
  import { renamedPath } from '../lib/tree'
  import type { TreeNode } from '../lib/tree'

  type Props = { node: TreeNode; depth: number; expanded: Record<string, boolean>; forceOpen?: boolean }
  let { node, depth, expanded = $bindable(), forceOpen = false }: Props = $props()

  const open = $derived(forceOpen || expanded[node.path] !== false)
  const activePath = $derived(app.active?.type === 'request' && app.active.source === 'workspace' ? app.active.path : null)
  let menu = $state(false)

  function label(n: TreeNode) {
    if (n.kind === 'grpc') return 'gRPC'
    if (n.kind === 'ws') return 'WS'
    return n.method === 'DELETE' ? 'DEL' : (n.method ?? 'GET')
  }

  async function rename() {
    menu = false
    const name = await promptText('Rename', 'Name', node.name, 'Rename')
    if (!name || name === node.name) return
    const to = renamedPath(node.path, name, node.isFolder)
    try {
      if (!node.isFolder) {
        const r = await api.loadRequest(node.path)
        r.name = name
        await api.saveRequest(node.path, r)
        const open = app.tabs.find((t) => t.type === 'request' && t.source === 'workspace' && t.path === node.path)
        if (open && open.type === 'request') {
          open.request.name = name
          open.saved = JSON.stringify(open.request)
        }
      }
      if (to !== node.path) {
        await api.movePath(node.path, to)
        app.retarget(node.path, to)
      }
      await app.refreshState()
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function remove() {
    menu = false
    const what = node.isFolder ? `folder "${node.name}" and everything in it` : `"${node.name}"`
    if (!(await confirmAction('Delete', `Delete ${what}? (You can restore it with git if it was committed.)`))) return
    try {
      await api.deletePath(node.path)
      app.closeUnder(node.path)
      await app.refreshState()
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function duplicate() {
    menu = false
    try {
      const r = await api.loadRequest(node.path)
      r.name = r.name + ' copy'
      const folder = node.path.includes('/') ? node.path.slice(0, node.path.lastIndexOf('/')) : ''
      const path = await api.createRequest(folder, r.name, r.kind)
      await api.saveRequest(path, r)
      await app.refreshState()
      await app.openRequest(path)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  // Drag a request/folder onto a folder to move it.
  function ondragstart(e: DragEvent) {
    e.dataTransfer?.setData('text/x-apiclient-path', node.path)
  }
  async function ondrop(e: DragEvent) {
    if (!node.isFolder) return
    e.preventDefault()
    const from = e.dataTransfer?.getData('text/x-apiclient-path')
    if (!from || from === node.path || node.path.startsWith(from + '/')) return
    const leaf = from.split('/').pop()!
    const to = node.path ? `${node.path}/${leaf}` : leaf
    try {
      await api.movePath(from, to)
      app.retarget(from, to)
      await app.refreshState()
    } catch (err) {
      toast(errMsg(err), 'error')
    }
  }
</script>

{#if node.path !== ''}
  <div
    class="item"
    class:active={activePath === node.path}
    style="padding-left: {8 + depth * 12}px"
    draggable="true"
    {ondragstart}
    ondragover={(e) => node.isFolder && e.preventDefault()}
    {ondrop}
    role="treeitem"
    aria-selected={activePath === node.path}
    tabindex="0"
    onclick={() => (node.isFolder ? (expanded[node.path] = !open) : app.openRequest(node.path))}
    onkeydown={(e) => e.key === 'Enter' && (node.isFolder ? (expanded[node.path] = !open) : app.openRequest(node.path))}
  >
    {#if node.isFolder}
      <span class="chev">{open ? '▾' : '▸'}</span>
      <span class="name">{node.name}</span>
    {:else}
      <span class="method m-{node.kind === 'http' || !node.kind ? node.method : node.kind}">{label(node)}</span>
      <span class="name">{node.name}</span>
    {/if}
    <span class="actions">
      {#if node.isFolder}
        <button class="icon-btn" title="New HTTP request here" onclick={(e) => { e.stopPropagation(); app.newRequest(node.path, 'http') }}>+</button>
      {/if}
      <button class="icon-btn" title="More" onclick={(e) => { e.stopPropagation(); menu = !menu }}>⋯</button>
    </span>
  </div>
  {#if menu}
    <div class="menu" style="margin-left: {20 + depth * 12}px" role="menu" tabindex="-1" onmouseleave={() => (menu = false)}>
      {#if node.isFolder}
        <button onclick={() => { menu = false; app.newRequest(node.path, 'http') }}>New HTTP request</button>
        <button onclick={() => { menu = false; app.newRequest(node.path, 'grpc') }}>New gRPC call</button>
        <button onclick={() => { menu = false; app.newRequest(node.path, 'ws') }}>New WebSocket</button>
        <button onclick={() => { menu = false; app.newFolder(node.path) }}>New folder</button>
        <button onclick={() => { menu = false; app.openFolderSettings(node.path) }}>Folder settings (headers, auth)</button>
      {:else}
        <button onclick={duplicate}>Duplicate</button>
      {/if}
      <button onclick={rename}>Rename</button>
      <button class="danger" onclick={remove}>Delete</button>
    </div>
  {/if}
{/if}
{#if node.isFolder && open}
  {#each node.children ?? [] as child (child.path)}
    <TreeItem node={child} depth={node.path === '' ? 0 : depth + 1} bind:expanded {forceOpen} />
  {/each}
{/if}

<style>
  .item {
    display: flex; align-items: center; gap: 6px; padding: 3px 6px; cursor: pointer; border-radius: 4px; margin: 0 4px;
    white-space: nowrap; outline: none;
  }
  .item:hover, .item:focus-visible { background: var(--bg-3); }
  .item.active { background: var(--bg-3); box-shadow: inset 2px 0 0 var(--accent); }
  .chev { width: 10px; color: var(--text-2); font-size: 10px; }
  .method { width: 34px; flex-shrink: 0; font-size: 10px; text-align: right; }
  .name { overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .actions { display: none; }
  .item:hover .actions { display: flex; }
  .menu {
    display: flex; flex-direction: column; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 4px; margin-right: 8px; box-shadow: 0 4px 16px #0003; position: relative; z-index: 5;
  }
  .menu button { background: none; border: none; text-align: left; padding: 5px 8px; border-radius: 4px; cursor: pointer; }
  .menu button:hover { background: var(--bg-3); }
  .menu .danger { color: var(--error); }
</style>
