<script lang="ts">
  import { api, errMsg, newId, on } from '../lib/api'
  import { app, isDirty, type RequestTab } from '../lib/state.svelte'
  import { promptText, toast } from '../lib/dialog.svelte'
  import { syncUrlParams } from '../lib/url'
  import { insertOrdered } from '../lib/stream'
  import type { MethodInfo, Preview, Request, StreamEvent } from '../lib/types'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
  import VarInput from './VarInput.svelte'
  import KVTable from './KVTable.svelte'
  import AuthEditor from './AuthEditor.svelte'
  import BodyEditor from './BodyEditor.svelte'
  import CodeEditor from './CodeEditor.svelte'
  import ResponseView from './ResponseView.svelte'
  import StreamLog from './StreamLog.svelte'

  // Look the tab up in the global store (rather than mutating a prop).
  let { id }: { id: string } = $props()
  const tab = $derived(app.tabs.find((t) => t.id === id) as RequestTab)

  const req = $derived(tab.request)
  const kind = $derived(req.kind)
  const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

  type Sub = 'params' | 'headers' | 'body' | 'auth' | 'settings' | 'docs' | 'message' | 'protos'
  let sub = $state<Sub>('params')
  $effect(() => {
    // Sensible default sub-tab per protocol when switching tabs.
    tab.id
    sub = kind === 'http' ? 'params' : 'message'
  })

  // ---- live preview (resolved URL + missing vars) ----
  let preview = $state<Preview | null>(null)
  const missing = $derived(preview?.missing ?? [])
  $effect(() => {
    const snap = JSON.stringify(tab.request)
    const path = tab.path
    const env = app.state.activeEnv
    const h = setTimeout(async () => {
      try {
        preview = await api.preview(path, JSON.parse(snap))
      } catch {
        preview = null
      }
    }, 200)
    return () => {
      clearTimeout(h)
      void env
    }
  })

  // ---- scratch autosave ----
  $effect(() => {
    if (tab.source !== 'scratch') return
    JSON.stringify(tab.request)
    const h = setTimeout(() => app.persistScratch(tab), 600)
    return () => clearTimeout(h)
  })

  const plain = (): Request => $state.snapshot(tab.request) as Request

  // ---- URL handling ----
  function onUrlInput(v: string) {
    if (kind === 'http' && v.includes('?')) {
      const r = syncUrlParams(v, tab.request.params ?? [])
      tab.request.url = r.url
      tab.request.params = r.params
    }
  }

  async function onUrlPaste(e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text') ?? ''
    if (!/^\s*curl\s/.test(text)) return
    e.preventDefault()
    try {
      const r = await api.importCurl(text)
      const name = tab.request.name
      tab.request = { ...r, name, kind: 'http', params: r.params ?? [], headers: r.headers ?? [] }
      toast('Imported cURL command')
    } catch (err) {
      toast(errMsg(err), 'error')
    }
  }

  // ---- HTTP ----
  async function send() {
    if (kind === 'ws') return tab.connected ? disconnect() : connect()
    if (kind === 'grpc') return tab.connected ? api.grpcCancel(tab.sessionId) : invoke()
    if (tab.running) {
      api.cancelRun(tab.runId)
      return
    }
    tab.running = true
    tab.error = ''
    tab.runId = newId()
    try {
      tab.result = await api.sendHTTP(tab.runId, tab.path, plain())
    } catch (e) {
      tab.error = errMsg(e)
      tab.result = null
    } finally {
      tab.running = false
      tab.runId = ''
    }
  }

  // ---- WebSocket ----
  let composer = $state('')

  // Events can be delivered out of order, so insert by seq. We keep
  // listening after the session ends (until the next one starts) so late
  // events aren't dropped.
  function listen(prefix: string, onEvent?: (e: StreamEvent) => void) {
    tab.unlisten?.()
    const session = newId()
    tab.sessionId = session
    tab.unlisten = on<StreamEvent>(prefix + session, (e) => {
      e.session = session
      insertOrdered(tab.log, e)
      onEvent?.(e)
    })
  }

  function endSession() {
    tab.connected = false
  }

  async function connect() {
    listen('ws:', (e) => e.type === 'close' && endSession())
    tab.connected = true
    try {
      await api.wsConnect(tab.sessionId, tab.path, plain())
    } catch (e) {
      endSession()
      tab.log.push({ type: 'error', data: errMsg(e), time: new Date().toISOString() })
    }
  }

  function disconnect() {
    api.wsClose(tab.sessionId)
  }

  async function sendWs(text = composer) {
    try {
      await api.wsSend(tab.sessionId, text)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function saveWsMessage() {
    const name = await promptText('Save message', 'Name', '')
    if (!name) return
    tab.request.ws ??= {}
    tab.request.ws.messages = [...(tab.request.ws.messages ?? []), { name, content: composer }]
  }

  // ---- gRPC ----
  let loadingServices = $state(false)
  const service = $derived(tab.services?.find((s) => s.name === req.grpc?.service))
  const method = $derived<MethodInfo | undefined>(service?.methods.find((m) => m.name === req.grpc?.method))
  const streamingClient = $derived(method?.clientStreaming ?? false)

  async function loadServices() {
    loadingServices = true
    try {
      tab.services = (await api.grpcDescribe(tab.path, plain())) ?? []
      if (!tab.services.length) toast('Server exposes no services', 'error')
    } catch (e) {
      toast(errMsg(e), 'error')
    } finally {
      loadingServices = false
    }
  }

  function pickMethod(svc: string, m: string) {
    tab.request.grpc ??= {}
    tab.request.grpc.service = svc
    tab.request.grpc.method = m
    const info = tab.services?.find((s) => s.name === svc)?.methods.find((x) => x.name === m)
    const cur = (tab.request.body?.content ?? '').trim()
    if (info && (cur === '' || cur === '{}')) {
      tab.request.body = { type: 'json', content: info.template }
    }
  }

  async function invoke() {
    listen('grpc:', (e) => e.type === 'end' && endSession())
    tab.connected = true
    try {
      await api.grpcStart(tab.sessionId, tab.path, plain())
    } catch (e) {
      endSession()
      tab.log.push({ type: 'error', data: errMsg(e), time: new Date().toISOString() })
    }
  }

  async function grpcSendMore() {
    try {
      await api.grpcSend(tab.sessionId, tab.request.body?.content ?? '{}')
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function addProto(field: 'protoFiles' | 'importPaths') {
    const p = field === 'protoFiles' ? await api.pickFile('Choose a .proto file', '*.proto') : await api.pickDirectory('Choose an import path')
    if (!p) return
    const root = app.state.workspace?.root
    const rel = root && p.startsWith(root + '/') ? p.slice(root.length + 1) : p
    tab.request.grpc ??= {}
    tab.request.grpc[field] = [...(tab.request.grpc[field] ?? []), rel]
    tab.services = null
  }

  // ---- misc ----
  async function copyCurl(resolved: boolean) {
    try {
      await ClipboardSetText(await api.exportCurl(tab.path, plain(), resolved))
      toast(resolved ? 'cURL (with resolved variables) copied' : 'cURL copied')
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  function onKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      send()
    }
  }

  const count = (rows?: { disabled?: boolean }[]) => (rows ?? []).filter((r) => !r.disabled).length

  function ensureSettings() {
    tab.request.settings ??= {}
    return tab.request.settings
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="editor">
  <div class="top row">
    <input class="name" bind:value={tab.request.name} spellcheck="false" />
    {#if tab.source === 'scratch'}<span class="pill" title="Not part of any collection">scratch</span>{/if}
    {#if tab.source === 'workspace'}<span class="muted mono path">{tab.path}</span>{/if}
    <span class="spacer"></span>
    {#if kind === 'http'}
      <button class="btn sm ghost" onclick={() => copyCurl(false)} title="Copy as cURL with {'{{variables}}'}">Copy cURL</button>
      <button class="btn sm ghost" onclick={() => copyCurl(true)} title="Copy as cURL with variables resolved (includes secrets!)">Copy resolved</button>
    {/if}
    {#if tab.source === 'scratch'}
      <button class="btn sm" onclick={() => app.promote(tab)}>Save to workspace…</button>
    {:else}
      <button class="btn sm" disabled={!isDirty(tab)} onclick={() => app.save(tab)}>Save</button>
    {/if}
  </div>

  <div class="urlbar row">
    {#if kind === 'http'}
      <select class="input method-select method m-{req.method}" bind:value={tab.request.method}>
        {#each methods as m}<option value={m}>{m}</option>{/each}
      </select>
    {:else}
      <span class="kind-badge method m-{kind}">{kind === 'ws' ? 'WS' : 'gRPC'}</span>
    {/if}
    <VarInput
      bind:value={tab.request.url}
      class="grow"
      {missing}
      placeholder={kind === 'grpc' ? 'localhost:50051  or  grpcs://api.example.com' : kind === 'ws' ? 'wss://example.com/socket' : 'https://api.example.com/items  — or paste a cURL command'}
      oninput={onUrlInput}
      onpaste={onUrlPaste}
    />
    {#if kind === 'grpc'}
      <label class="row muted plain" title="Use an insecure (non-TLS) connection">
        <input type="checkbox" checked={req.grpc?.plaintext ?? false} onchange={(e) => { tab.request.grpc ??= {}; tab.request.grpc.plaintext = (e.target as HTMLInputElement).checked || undefined }} /> plaintext
      </label>
    {/if}
    <button class="btn primary send" onclick={send}>
      {#if kind === 'http'}{tab.running ? 'Cancel' : 'Send'}
      {:else if kind === 'ws'}{tab.connected ? 'Disconnect' : 'Connect'}
      {:else}{tab.connected ? 'Cancel' : 'Invoke'}{/if}
    </button>
  </div>

  {#if kind === 'grpc'}
    <div class="grpcbar row">
      <button class="btn sm" onclick={loadServices} disabled={loadingServices}>
        {loadingServices ? 'Loading…' : tab.services ? 'Reload methods' : req.grpc?.protoFiles?.length ? 'Load from protos' : 'Load via reflection'}
      </button>
      {#if tab.services}
        <select class="input grow" value={req.grpc?.service && req.grpc?.method ? `${req.grpc.service}/${req.grpc.method}` : ''}
          onchange={(e) => { const [s, m] = (e.target as HTMLSelectElement).value.split('/'); pickMethod(s, m) }}>
          <option value="" disabled>Choose a method…</option>
          {#each tab.services as s}
            <optgroup label={s.name}>
              {#each s.methods as m}
                <option value="{s.name}/{m.name}">{m.name}{m.clientStreaming && m.serverStreaming ? ' (bidi stream)' : m.clientStreaming ? ' (client stream)' : m.serverStreaming ? ' (server stream)' : ''}</option>
              {/each}
            </optgroup>
          {/each}
        </select>
      {:else}
        <input class="input mono grow" placeholder="package.Service" value={req.grpc?.service ?? ''} oninput={(e) => { tab.request.grpc ??= {}; tab.request.grpc.service = (e.target as HTMLInputElement).value }} />
        <input class="input mono" style="width: 180px" placeholder="Method" value={req.grpc?.method ?? ''} oninput={(e) => { tab.request.grpc ??= {}; tab.request.grpc.method = (e.target as HTMLInputElement).value }} />
      {/if}
      {#if tab.connected && streamingClient}
        <button class="btn sm" onclick={grpcSendMore} title="Send the message editor's content on the open stream">Send message</button>
        <button class="btn sm" onclick={() => api.grpcCloseSend(tab.sessionId)}>End stream</button>
      {/if}
    </div>
  {/if}

  {#if preview?.error}
    <div class="warnbox pv">{preview.error}</div>
  {:else if missing.length}
    <div class="warnbox pv">Undefined variables: {missing.join(', ')} {app.state.activeEnv ? '' : '— no environment selected'}</div>
  {:else if preview?.url && preview.url !== req.url && kind === 'http'}
    <div class="pv muted mono resolved" title="Resolved URL (secrets masked)">→ {preview.url}</div>
  {/if}

  <div class="split">
    <div class="left">
      <div class="subtabs">
        {#if kind === 'http'}
          <button class:on={sub === 'params'} onclick={() => (sub = 'params')}>Params{#if count(req.params)}<span class="count">{count(req.params)}</span>{/if}</button>
          <button class:on={sub === 'body'} onclick={() => (sub = 'body')}>Body{#if req.body && req.body.type !== 'none'}<span class="count">●</span>{/if}</button>
        {:else}
          <button class:on={sub === 'message'} onclick={() => (sub = 'message')}>{kind === 'ws' ? 'Messages' : 'Message'}</button>
          {#if kind === 'ws'}
            <button class:on={sub === 'params'} onclick={() => (sub = 'params')}>Params{#if count(req.params)}<span class="count">{count(req.params)}</span>{/if}</button>
          {/if}
        {/if}
        <button class:on={sub === 'headers'} onclick={() => (sub = 'headers')}>{kind === 'grpc' ? 'Metadata' : 'Headers'}{#if count(req.headers)}<span class="count">{count(req.headers)}</span>{/if}</button>
        <button class:on={sub === 'auth'} onclick={() => (sub = 'auth')}>Auth</button>
        {#if kind === 'grpc'}<button class:on={sub === 'protos'} onclick={() => (sub = 'protos')}>Protos{#if req.grpc?.protoFiles?.length}<span class="count">{req.grpc.protoFiles.length}</span>{/if}</button>{/if}
        <button class:on={sub === 'settings'} onclick={() => (sub = 'settings')}>Settings</button>
        <button class:on={sub === 'docs'} onclick={() => (sub = 'docs')}>Docs</button>
      </div>
      <div class="pane">
        {#if sub === 'params'}
          <KVTable bind:rows={tab.request.params} {missing} />
        {:else if sub === 'headers'}
          <KVTable bind:rows={tab.request.headers} {missing} keyPlaceholder={kind === 'grpc' ? 'metadata-key' : 'Header'} />
        {:else if sub === 'body'}
          <BodyEditor bind:body={tab.request.body} {missing} />
        {:else if sub === 'auth'}
          <AuthEditor bind:auth={tab.request.auth} {missing} allowInherit={tab.source === 'workspace'} />
        {:else if sub === 'message' && kind === 'grpc'}
          <div class="fill">
            {#if method}<div class="hint muted mono">{method.inputType} → {method.outputType}</div>{/if}
            <div class="grow-pane">
              <CodeEditor bind:value={() => tab.request.body?.content ?? '', (v) => (tab.request.body = { type: 'json', content: v })} language="json" />
            </div>
          </div>
        {:else if sub === 'message' && kind === 'ws'}
          <div class="fill">
            <div class="grow-pane"><CodeEditor bind:value={composer} language="json" placeholder="Message to send…" /></div>
            <div class="row composer-actions">
              <button class="btn primary sm" disabled={!tab.connected} onclick={() => sendWs()}>Send</button>
              <button class="btn sm ghost" disabled={!composer} onclick={saveWsMessage}>Save message</button>
            </div>
            {#if req.ws?.messages?.length}
              <div class="saved">
                <div class="section-title">Saved messages</div>
                {#each req.ws.messages as m, i (i)}
                  <div class="row saved-item">
                    <button class="btn sm ghost grow left-align" onclick={() => (composer = m.content)} title="Load into editor">{m.name}</button>
                    <button class="btn sm" disabled={!tab.connected} onclick={() => sendWs(m.content)}>Send</button>
                    <button class="icon-btn" onclick={() => tab.request.ws?.messages?.splice(i, 1)}>✕</button>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {:else if sub === 'protos'}
          <div class="padded col">
            <p class="muted">Without proto files, methods are discovered with server reflection. Paths are relative to the workspace root so they work for everyone who clones the repo.</p>
            <div class="section-title">Proto files</div>
            {#each req.grpc?.protoFiles ?? [] as f, i (i)}
              <div class="row"><span class="mono grow">{f}</span><button class="icon-btn" onclick={() => { tab.request.grpc?.protoFiles?.splice(i, 1); tab.services = null }}>✕</button></div>
            {/each}
            <div><button class="btn sm" onclick={() => addProto('protoFiles')}>Add .proto file…</button></div>
            <div class="section-title">Import paths</div>
            {#each req.grpc?.importPaths ?? [] as f, i (i)}
              <div class="row"><span class="mono grow">{f}</span><button class="icon-btn" onclick={() => tab.request.grpc?.importPaths?.splice(i, 1)}>✕</button></div>
            {/each}
            <div><button class="btn sm" onclick={() => addProto('importPaths')}>Add import path…</button></div>
          </div>
        {:else if sub === 'settings'}
          <div class="padded col settings">
            {#if kind === 'http'}
              <label class="row"><span class="lbl">Timeout (ms)</span>
                <input class="input" type="number" min="0" placeholder="60000" value={req.settings?.timeoutMs ?? ''}
                  oninput={(e) => (ensureSettings().timeoutMs = Number((e.target as HTMLInputElement).value) || undefined)} /></label>
              <label class="row"><input type="checkbox" checked={req.settings?.followRedirects ?? true}
                onchange={(e) => (ensureSettings().followRedirects = (e.target as HTMLInputElement).checked ? undefined : false)} /> Follow redirects</label>
            {/if}
            <label class="row"><input type="checkbox" checked={req.settings?.insecureSkipVerify ?? false}
              onchange={(e) => (ensureSettings().insecureSkipVerify = (e.target as HTMLInputElement).checked || undefined)} /> Skip TLS certificate verification</label>
            {#if kind === 'ws'}
              <label class="row"><span class="lbl">Subprotocols</span>
                <input class="input mono grow" placeholder="graphql-ws, chat" value={(req.ws?.subprotocols ?? []).join(', ')}
                  onchange={(e) => { tab.request.ws ??= {}; const v = (e.target as HTMLInputElement).value.split(',').map((s) => s.trim()).filter(Boolean); tab.request.ws.subprotocols = v.length ? v : undefined }} /></label>
            {/if}
            {#if kind === 'http'}<div><button class="btn sm" onclick={() => api.clearCookies().then(() => toast('Cookies cleared'))}>Clear cookies</button></div>{/if}
          </div>
        {:else if sub === 'docs'}
          <textarea class="input docs" placeholder="Notes about this request (saved to the file)" bind:value={tab.request.docs}></textarea>
        {/if}
      </div>
    </div>
    <div class="right">
      {#if kind === 'http'}
        <ResponseView result={tab.result} error={tab.error} running={tab.running} />
      {:else}
        <StreamLog events={tab.log} status={tab.connected ? (kind === 'ws' ? 'Connected' : 'Call in progress') : 'Idle'} onclear={() => (tab.log = [])} />
      {/if}
    </div>
  </div>
</div>

<style>
  .editor { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .top { padding: 8px 12px 4px; }
  .name { font-size: 15px; font-weight: 600; border: 1px solid transparent; background: transparent; border-radius: 4px; padding: 2px 4px; min-width: 120px; outline: none; }
  .name:hover, .name:focus { border-color: var(--border); }
  .path { font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .urlbar { padding: 4px 12px; }
  .urlbar :global(.grow) { flex: 1; }
  .method-select { width: 100px; font-weight: 600; }
  .kind-badge { padding: 6px 10px; background: var(--bg-3); border-radius: var(--radius); }
  .send { min-width: 90px; }
  .plain { font-size: 12px; gap: 3px; }
  .grpcbar { padding: 4px 12px; }
  .pv { margin: 2px 12px 4px; }
  .resolved { font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .split { flex: 1; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); min-height: 0; border-top: 1px solid var(--border); margin-top: 4px; }
  .left { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--border); }
  .right { min-height: 0; display: flex; flex-direction: column; }
  .pane { flex: 1; min-height: 0; overflow: auto; }
  .fill { display: flex; flex-direction: column; height: 100%; }
  .grow-pane { flex: 1; min-height: 120px; }
  .hint { padding: 4px 10px; font-size: 11px; }
  .padded { padding: 10px; }
  .settings { max-width: 520px; }
  .lbl { width: 110px; color: var(--text-2); flex-shrink: 0; }
  .docs { width: 100%; height: 100%; border: none; border-radius: 0; resize: none; font-family: inherit; font-size: 13px; padding: 10px; }
  .composer-actions { padding: 6px 10px; border-top: 1px solid var(--border); }
  .saved { padding: 6px 10px; border-top: 1px solid var(--border); max-height: 40%; overflow: auto; }
  .saved-item { margin-top: 4px; }
  .left-align { text-align: left; }
  @media (max-width: 900px) {
    .split { grid-template-columns: 1fr; grid-template-rows: 1fr 1fr; }
    .left { border-right: none; border-bottom: 1px solid var(--border); }
  }
</style>
