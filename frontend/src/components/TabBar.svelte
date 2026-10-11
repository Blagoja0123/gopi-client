<script lang="ts">
  import { app, isDirty, type Tab } from '../lib/state.svelte'

  function title(t: Tab) {
    if (t.type === 'webhooks') return 'Webhooks'
    if (t.type === 'folder') return `⚙ ${t.path || 'root'}`
    return t.request.name || 'Untitled'
  }
  function badge(t: Tab) {
    if (t.type !== 'request') return ''
    if (t.request.kind === 'ws') return 'WS'
    if (t.request.kind === 'grpc') return 'gRPC'
    return t.request.method ?? 'GET'
  }
</script>

<div class="tabbar">
  {#each app.tabs as t (t.id)}
    <div class="tab" class:on={t.id === app.activeId} role="tab" tabindex="0" aria-selected={t.id === app.activeId}
      onclick={() => (app.activeId = t.id)}
      onkeydown={(e) => e.key === 'Enter' && (app.activeId = t.id)}
      onauxclick={(e) => e.button === 1 && app.closeTab(t.id)}>
      {#if badge(t)}<span class="method m-{t.type === 'request' && t.request.kind !== 'http' ? t.request.kind : badge(t)}">{badge(t)}</span>{/if}
      <span class="t" class:italic={t.type === 'request' && t.source === 'scratch'}>{title(t)}</span>
      {#if isDirty(t) && !(t.type === 'request' && t.source === 'scratch')}<span class="dot" title="Unsaved changes">●</span>{/if}
      <button class="icon-btn close" title="Close (Ctrl+W)" onclick={(e) => { e.stopPropagation(); app.closeTab(t.id) }}>✕</button>
    </div>
  {/each}
</div>

<style>
  .tabbar { display: flex; overflow-x: auto; background: var(--bg-2); border-bottom: 1px solid var(--border); flex-shrink: 0; min-height: 34px; }
  .tabbar::-webkit-scrollbar { height: 4px; }
  .tab {
    display: flex; align-items: center; gap: 6px; padding: 0 6px 0 12px; border-right: 1px solid var(--border); cursor: pointer;
    max-width: 220px; white-space: nowrap; color: var(--text-2);
  }
  .tab.on { background: var(--bg); color: var(--text); box-shadow: inset 0 2px 0 var(--accent); }
  .t { overflow: hidden; text-overflow: ellipsis; }
  .italic { font-style: italic; }
  .method { font-size: 10px; }
  .dot { color: var(--accent); font-size: 10px; }
  .close { visibility: hidden; font-size: 11px; }
  .tab:hover .close, .tab.on .close { visibility: visible; }
</style>
