<script lang="ts">
  import { onMount } from 'svelte'
  import { api, errMsg, on } from '../lib/api'
  import { app } from '../lib/state.svelte'
  import { promptFields, toast } from '../lib/dialog.svelte'
  import { formatBytes, formatDuration, prettyBody, statusTone } from '../lib/format'
  import type { Capture, HttpResponse, WebhookStatus } from '../lib/types'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

  let status = $state<WebhookStatus>({ running: false, addr: '' })
  let addr = $state('127.0.0.1:8090')
  let captures = $state<Capture[]>([])
  let selectedId = $state('')
  let respStatus = $state(200)
  let respType = $state('application/json')
  let respBody = $state('{"received":true}')
  let replayTarget = $state('http://localhost:3000')
  let replayResult = $state<HttpResponse | null>(null)
  let replayError = $state('')
  let verifyPreset = $state('github')
  let verifySecret = $state('')
  let verifyResult = $state<boolean | null>(null)
  // signer
  let signPreset = $state('github')
  let signSecret = $state('')
  let signPayload = $state('{"hello":"world"}')
  let signOut = $state<{ header: string; value: string } | null>(null)
  let view = $state<'inspect' | 'sign'>('inspect')

  const selected = $derived(captures.find((c) => c.id === selectedId) ?? null)
  const newestFirst = $derived([...captures].reverse())

  onMount(() => {
    api.webhookStatus().then((s) => (status = s))
    api.webhookCaptures().then((c) => (captures = c ?? []))
    return on<Capture>('webhook:capture', (c) => {
      captures.push(c)
      if (!selectedId) selectedId = c.id
    })
  })

  async function toggle() {
    try {
      if (status.running) {
        await api.webhookStop()
        status = { running: false, addr: '' }
      } else {
        await api.webhookSetResponse({ status: respStatus, contentType: respType, body: respBody })
        status = await api.webhookStart(addr)
      }
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function updateResponse() {
    await api.webhookSetResponse({ status: Number(respStatus) || 200, contentType: respType, body: respBody })
    toast('Response updated')
  }

  async function replay() {
    if (!selected) return
    replayError = ''
    replayResult = null
    try {
      replayResult = await api.webhookReplay(selected.id, replayTarget)
    } catch (e) {
      replayError = errMsg(e)
    }
  }

  async function saveAsRequest() {
    if (!selected) return
    if (!app.state.workspace) {
      toast('Open a workspace to save captures as requests', 'error')
      return
    }
    const r = await promptFields('Save webhook as request', [
      { label: 'Folder', value: '', options: app.folders.map((f) => ({ value: f, label: f || '/ (root)' })) },
      { label: 'Target base URL (variables allowed)', value: '{{webhookTarget}}' },
    ], 'Save')
    if (!r) return
    try {
      const path = await api.webhookSaveCapture(selected.id, r[0], r[1])
      await app.refreshState()
      await app.openRequest(path)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function verify() {
    if (!selected) return
    try {
      verifyResult = await api.verifySignature(verifyPreset, verifySecret, selected.id)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function sign() {
    try {
      signOut = await api.signPayload(signPreset, signSecret, signPayload)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  function url(c: Capture) {
    return c.path + (c.query ? '?' + c.query : '')
  }
</script>

<div class="wh">
  <div class="bar row">
    <h3>Webhooks</h3>
    <div class="subtabs inline">
      <button class:on={view === 'inspect'} onclick={() => (view = 'inspect')}>Receiver</button>
      <button class:on={view === 'sign'} onclick={() => (view = 'sign')}>Signature tool</button>
    </div>
    <span class="spacer"></span>
    {#if view === 'inspect'}
      <span class="dotlive" class:on={status.running}></span>
      {#if status.running}
        <span class="mono selectable">http://{status.addr}/</span>
        <button class="btn sm ghost" onclick={() => ClipboardSetText(`http://${status.addr}/`).then(() => toast('URL copied'))}>Copy</button>
      {:else}
        <input class="input mono" style="width: 170px" bind:value={addr} title="Listen address (use 0.0.0.0:PORT to accept from your network)" />
      {/if}
      <button class="btn sm" class:primary={!status.running} onclick={toggle}>{status.running ? 'Stop' : 'Start receiver'}</button>
    {/if}
  </div>

  {#if view === 'sign'}
    <div class="signer col">
      <p class="muted">Compute a signature for a payload, e.g. to add as a header on a request that simulates a webhook.</p>
      <div class="row">
        <select class="input" bind:value={signPreset}>
          <option value="github">GitHub (X-Hub-Signature-256)</option>
          <option value="stripe">Stripe (Stripe-Signature)</option>
          <option value="hmac-sha256">HMAC-SHA256 (hex)</option>
          <option value="hmac-sha1">HMAC-SHA1 (hex)</option>
          <option value="hmac-sha512">HMAC-SHA512 (hex)</option>
        </select>
        <input class="input mono grow" type="password" placeholder="secret" bind:value={signSecret} />
      </div>
      <textarea class="input" rows="8" bind:value={signPayload}></textarea>
      <div><button class="btn primary" onclick={sign}>Sign</button></div>
      {#if signOut}
        <div class="row out">
          <span class="mono muted">{signOut.header}:</span>
          <span class="mono grow selectable brk">{signOut.value}</span>
          <button class="btn sm" onclick={() => signOut && ClipboardSetText(signOut.value).then(() => toast('Copied'))}>Copy</button>
        </div>
        <p class="muted small">The payload must be sent byte-for-byte as signed.</p>
      {/if}
    </div>
  {:else}
    <div class="grid">
      <div class="list">
        <div class="list-head row">
          <span class="muted">{captures.length} captured</span>
          <span class="spacer"></span>
          <button class="btn sm ghost" onclick={() => api.webhookClear().then(() => { captures = []; selectedId = '' })}>Clear</button>
        </div>
        {#each newestFirst as c (c.id)}
          <div class="cap" class:on={c.id === selectedId} role="button" tabindex="0" onclick={() => { selectedId = c.id; replayResult = null; verifyResult = null }}
            onkeydown={(e) => e.key === 'Enter' && (selectedId = c.id)}>
            <div class="row"><span class="method m-{c.method}">{c.method}</span><span class="mono grow ell">{url(c)}</span></div>
            <div class="muted small">{new Date(c.time).toLocaleTimeString()} · {formatBytes(c.size)}</div>
          </div>
        {:else}
          <div class="empty small">
            {#if status.running}
              Waiting for requests to <span class="mono">http://{status.addr}/</span>…
            {:else}
              Start the receiver, then point a webhook at it. To receive from the internet, expose the port with a tunnel, e.g.
              <code>cloudflared tunnel --url http://{addr}</code> or <code>ngrok http {addr.split(':').pop()}</code>.
            {/if}
          </div>
        {/each}
        <details class="respcfg">
          <summary class="muted">Receiver response</summary>
          <div class="col">
            <div class="row"><input class="input" type="number" style="width: 80px" bind:value={respStatus} /><input class="input grow" bind:value={respType} /></div>
            <textarea class="input" rows="3" bind:value={respBody}></textarea>
            <button class="btn sm" onclick={updateResponse}>Apply</button>
          </div>
        </details>
      </div>
      <div class="detail">
        {#if selected}
          <div class="row dhead">
            <span class="method m-{selected.method}">{selected.method}</span>
            <span class="mono selectable grow">{url(selected)}</span>
            <button class="btn sm" onclick={saveAsRequest}>Save as request…</button>
          </div>
          <div class="section-title">Headers</div>
          <table class="kv"><tbody>
            {#each selected.headers ?? [] as h}<tr><td class="k">{h.key}</td><td class="v">{h.value}</td></tr>{/each}
          </tbody></table>
          <div class="section-title">Body {selected.bodyBase64 ? '(base64)' : ''}</div>
          <pre class="mono selectable body">{selected.bodyBase64 ? selected.body : prettyBody(selected.body, '')}</pre>

          <div class="section-title">Replay</div>
          <div class="row">
            <input class="input mono grow" bind:value={replayTarget} placeholder="http://localhost:3000" />
            <button class="btn primary sm" onclick={replay}>Replay</button>
          </div>
          <p class="muted small">Sends the same method, path, headers (including signatures) and body to this base URL.</p>
          {#if replayError}<div class="warnbox">{replayError}</div>{/if}
          {#if replayResult}
            <div class="row"><span class="tone-{statusTone(replayResult.status)} mono">{replayResult.statusText}</span><span class="pill">{formatDuration(replayResult.durationMs)}</span></div>
            {#if replayResult.body}<pre class="mono selectable body">{prettyBody(replayResult.body, replayResult.contentType)}</pre>{/if}
          {/if}

          <div class="section-title">Verify signature</div>
          <div class="row">
            <select class="input" bind:value={verifyPreset}>
              <option value="github">GitHub</option>
              <option value="stripe">Stripe</option>
            </select>
            <input class="input mono grow" type="password" placeholder="webhook secret" bind:value={verifySecret} />
            <button class="btn sm" onclick={verify}>Verify</button>
            {#if verifyResult !== null}
              <span class:tone-ok={verifyResult} class:tone-error={!verifyResult}>{verifyResult ? '✓ valid' : '✗ invalid'}</span>
            {/if}
          </div>
        {:else}
          <div class="empty">Select a captured request.</div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .wh { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .bar { padding: 8px 12px; border-bottom: 1px solid var(--border); }
  h3 { margin: 0 12px 0 0; font-size: 15px; }
  .subtabs.inline { border: none; padding: 0; }
  .dotlive { width: 8px; height: 8px; border-radius: 50%; background: var(--text-2); }
  .dotlive.on { background: var(--ok); box-shadow: 0 0 0 3px var(--var-ok-bg); }
  .grid { flex: 1; display: grid; grid-template-columns: 300px 1fr; min-height: 0; }
  .list { border-right: 1px solid var(--border); overflow: auto; display: flex; flex-direction: column; }
  .list-head { padding: 6px 10px; border-bottom: 1px solid var(--border); }
  .cap { padding: 6px 10px; border-bottom: 1px solid var(--border); cursor: pointer; }
  .cap:hover, .cap.on { background: var(--bg-3); }
  .ell { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .small { font-size: 11px; }
  .respcfg { margin-top: auto; padding: 8px 10px; border-top: 1px solid var(--border); }
  .respcfg summary { cursor: pointer; margin-bottom: 6px; }
  .detail { overflow: auto; padding: 10px 14px; display: flex; flex-direction: column; gap: 8px; }
  .dhead { font-size: 13px; }
  .section-title { margin-top: 8px; }
  .body { background: var(--bg-2); padding: 8px; border-radius: var(--radius); white-space: pre-wrap; word-break: break-all; margin: 0; max-height: 360px; overflow: auto; }
  .signer { padding: 14px; max-width: 760px; gap: 10px; }
  .signer p { margin: 0; }
  .out { background: var(--bg-2); padding: 8px; border-radius: var(--radius); }
  .brk { word-break: break-all; }
  code { font-family: var(--mono); font-size: 11px; background: var(--bg-3); padding: 0 3px; border-radius: 3px; }
  .empty code { user-select: text; }
</style>
