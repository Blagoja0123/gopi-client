<script lang="ts">
  import { api, errMsg } from '../lib/api'
  import { app } from '../lib/state.svelte'
  import { toast } from '../lib/dialog.svelte'

  let mode = $state<'openapi' | 'curl'>(app.state.workspace ? 'openapi' : 'curl')
  let url = $state('')
  let curl = $state('')
  let busy = $state(false)
  let error = $state('')

  async function run(fn: () => Promise<void>) {
    busy = true
    error = ''
    try {
      await fn()
    } catch (e) {
      error = errMsg(e)
    } finally {
      busy = false
    }
  }

  async function done(folder: string) {
    await app.refreshState()
    app.importOpen = false
    toast(`Imported into ${folder}/`)
  }

  const fromFile = () =>
    run(async () => {
      const p = await api.pickFile('Choose an OpenAPI / Swagger file', '*.yaml;*.yml;*.json')
      if (p) await done(await api.importOpenAPIFile(p))
    })

  const fromUrl = () => run(async () => done(await api.importOpenAPIURL(url)))

  const fromCurl = () =>
    run(async () => {
      const r = await api.importCurl(curl)
      await app.newScratch('http', r)
      app.importOpen = false
    })
</script>

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && (app.importOpen = false)}>
  <div class="modal" role="dialog" aria-modal="true">
    <div class="head row">
      <h3>Import</h3>
      <span class="spacer"></span>
      <button class="icon-btn" onclick={() => (app.importOpen = false)}>✕</button>
    </div>
    <div class="subtabs">
      <button class:on={mode === 'openapi'} onclick={() => (mode = 'openapi')}>OpenAPI / Swagger</button>
      <button class:on={mode === 'curl'} onclick={() => (mode = 'curl')}>cURL</button>
    </div>
    <div class="body col">
      {#if mode === 'openapi'}
        {#if !app.state.workspace}
          <div class="warnbox">Open or create a workspace first — imported requests are saved as files in it.</div>
        {:else}
          <p class="muted">Creates a folder of requests (grouped by tag) in this workspace. The server URL becomes a workspace variable, and the spec's security scheme becomes the folder's auth (e.g. <code>{'{{token}}'}</code>).</p>
          <button class="btn" disabled={busy} onclick={fromFile}>Choose file…</button>
          <div class="row">
            <input class="input grow" placeholder="https://petstore3.swagger.io/api/v3/openapi.json" bind:value={url} onkeydown={(e) => e.key === 'Enter' && url && fromUrl()} />
            <button class="btn primary" disabled={busy || !url} onclick={fromUrl}>Import URL</button>
          </div>
        {/if}
      {:else}
        <p class="muted">Paste a cURL command (e.g. from browser dev tools → Copy as cURL). It opens as a scratch request. Tip: you can also paste cURL straight into any URL bar.</p>
        <textarea class="input" rows="8" placeholder="curl 'https://api.example.com' -H 'accept: application/json'" bind:value={curl}></textarea>
        <div class="row"><span class="spacer"></span><button class="btn primary" disabled={busy || !curl.trim()} onclick={fromCurl}>Import</button></div>
      {/if}
      {#if error}<div class="warnbox selectable">{error}</div>{/if}
    </div>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: #0006; display: flex; align-items: center; justify-content: center; z-index: 50; }
  .modal { width: min(620px, 94vw); background: var(--bg); border-radius: 10px; border: 1px solid var(--border); box-shadow: 0 20px 60px #0006; }
  .head { padding: 12px 16px 6px; }
  h3 { margin: 0; font-size: 16px; }
  .body { padding: 14px 16px 16px; gap: 10px; }
  .body .btn { align-self: flex-start; }
  .body .row .btn { align-self: auto; }
  p { margin: 0; }
  code { font-family: var(--mono); font-size: 11px; }
</style>
