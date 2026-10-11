<script lang="ts">
  import { api, errMsg } from '../lib/api'
  import { app } from '../lib/state.svelte'
  import { confirmAction, promptText, toast } from '../lib/dialog.svelte'
  import type { Variable } from '../lib/types'

  const GLOBALS = '\u0000globals'
  let selected = $state(app.state.activeEnv || GLOBALS)
  let vars = $state<Variable[]>([])
  let saved = $state('')
  let reveal = $state<Record<number, boolean>>({})
  const dirty = $derived(JSON.stringify(vars) !== saved)
  const isGlobals = $derived(selected === GLOBALS)

  async function load(name: string) {
    try {
      const v = name === GLOBALS ? ((await api.getGlobals()) ?? []) : ((await api.getEnvironment(name)).variables ?? [])
      vars = v.map((x) => ({ ...x, value: x.value ?? '' }))
      saved = JSON.stringify(vars)
      reveal = {}
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  $effect(() => {
    load(selected)
  })

  async function select(name: string) {
    if (dirty && !(await confirmAction('Discard changes?', 'You have unsaved variable changes.', 'Discard'))) return
    selected = name
  }

  async function save() {
    const clean = vars.filter((v) => v.key.trim() !== '')
    try {
      if (isGlobals) await api.saveGlobals(clean)
      else await api.saveEnvironment({ name: selected, variables: clean })
      vars = clean
      saved = JSON.stringify(vars)
      toast('Saved')
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function create() {
    const name = await promptText('New environment', 'Name (e.g. dev, staging, prod)', '', 'Create')
    if (!name) return
    try {
      await api.saveEnvironment({ name, variables: [] })
      await app.refreshState()
      selected = name
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async function remove() {
    if (!(await confirmAction('Delete environment', `Delete "${selected}" (including its local secrets)?`))) return
    await api.deleteEnvironment(selected)
    await app.refreshState()
    selected = GLOBALS
  }

  function close() {
    if (dirty) {
      confirmAction('Discard changes?', 'You have unsaved variable changes.', 'Discard').then((ok) => ok && (app.envEditor = false))
    } else app.envEditor = false
  }
</script>

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && close()}>
  <div class="modal" role="dialog" aria-modal="true">
    <div class="list">
      <div class="section-title">Variables</div>
      <button class:on={isGlobals} onclick={() => select(GLOBALS)}>Workspace (all envs)</button>
      <div class="section-title" style="margin-top: 10px">Environments</div>
      {#each app.state.environments ?? [] as e}
        <button class:on={selected === e} onclick={() => select(e)}>
          {e}{#if e === app.state.activeEnv}<span class="pill act">active</span>{/if}
        </button>
      {/each}
      <button class="add" onclick={create}>+ New environment</button>
    </div>
    <div class="main">
      <div class="head row">
        <h3>{isGlobals ? 'Workspace variables' : selected}</h3>
        <span class="spacer"></span>
        {#if !isGlobals}
          {#if selected !== app.state.activeEnv}<button class="btn sm" onclick={() => app.setEnv(selected)}>Use this environment</button>{/if}
          <button class="btn sm danger" onclick={remove}>Delete</button>
        {/if}
        <button class="icon-btn" onclick={close}>✕</button>
      </div>
      <p class="muted info">
        {#if isGlobals}
          Stored in <code>apiclient.yaml</code> (committed). Used by every environment; environment values override these. Don't put secrets here.
        {:else}
          Stored in <code>environments/{selected}.yaml</code> (committed). Values of <b>secret</b> variables go to
          <code>environments/{selected}.secret.yaml</code>, which is gitignored — teammates see the variable name but set their own value.
        {/if}
      </p>
      <div class="table">
        <div class="r hdr muted">
          <span class="cb"></span><span class="grow">Name</span><span class="grow2">Value</span>
          {#if !isGlobals}<span class="sec">Secret</span>{/if}<span class="x"></span>
        </div>
        {#each vars as v, i (i)}
          <div class="r" class:off={v.disabled}>
            <input type="checkbox" class="cb" checked={!v.disabled} onchange={(e) => (v.disabled = !(e.target as HTMLInputElement).checked || undefined)} />
            <input class="input mono grow" bind:value={v.key} placeholder="name" spellcheck="false" />
            <div class="grow2 row">
              <input class="input mono grow" type={v.secret && !reveal[i] ? 'password' : 'text'} bind:value={v.value} placeholder={v.secret ? 'secret value (local only)' : 'value'} spellcheck="false" />
              {#if v.secret}<button class="icon-btn" title="Show/hide" onclick={() => (reveal[i] = !reveal[i])}>{reveal[i] ? '◉' : '◎'}</button>{/if}
            </div>
            {#if !isGlobals}
              <input type="checkbox" class="sec" checked={v.secret ?? false} onchange={(e) => (v.secret = (e.target as HTMLInputElement).checked || undefined)} title="Keep the value out of git" />
            {/if}
            <button class="icon-btn x" onclick={() => vars.splice(i, 1)}>✕</button>
          </div>
        {/each}
        <button class="btn sm add-var" onclick={() => vars.push({ key: '', value: '' })}>+ Add variable</button>
      </div>
      <div class="foot row">
        <span class="muted">Reference variables as <code>{'{{name}}'}</code>. Built-ins: <code>{'{{$uuid}}'}</code> <code>{'{{$timestamp}}'}</code> <code>{'{{$isoTimestamp}}'}</code> <code>{'{{$randomInt}}'}</code></span>
        <span class="spacer"></span>
        <button class="btn primary" disabled={!dirty} onclick={save}>Save</button>
      </div>
    </div>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: #0006; display: flex; align-items: center; justify-content: center; z-index: 50; }
  .modal { width: min(920px, 94vw); height: min(620px, 90vh); background: var(--bg); border-radius: 10px; display: flex; overflow: hidden; border: 1px solid var(--border); box-shadow: 0 20px 60px #0006; }
  .list { width: 210px; background: var(--bg-2); border-right: 1px solid var(--border); padding: 12px 8px; display: flex; flex-direction: column; gap: 2px; overflow: auto; }
  .list button { background: none; border: none; text-align: left; padding: 6px 8px; border-radius: 4px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; }
  .list button:hover, .list button.on { background: var(--bg-3); }
  .list .add { color: var(--accent); }
  .list .section-title { padding: 0 8px 4px; }
  .act { font-size: 10px; background: var(--var-ok-bg); color: var(--var-ok); }
  .main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
  .head { padding: 12px 16px 0; }
  h3 { margin: 0; font-size: 16px; }
  .info { padding: 0 16px; font-size: 12px; }
  .table { flex: 1; overflow: auto; padding: 0 16px; display: flex; flex-direction: column; gap: 4px; }
  .r { display: flex; align-items: center; gap: 8px; }
  .r.off input:not([type='checkbox']) { opacity: 0.5; }
  .hdr { font-size: 11px; position: sticky; top: 0; background: var(--bg); padding: 4px 0; }
  .cb { width: 16px; flex-shrink: 0; }
  .grow { flex: 1; min-width: 0; }
  .grow2 { flex: 1.5; min-width: 0; }
  .sec { width: 44px; flex-shrink: 0; text-align: center; }
  .x { width: 22px; flex-shrink: 0; }
  .add-var { align-self: flex-start; margin-top: 4px; }
  .foot { padding: 12px 16px; border-top: 1px solid var(--border); font-size: 12px; }
  code { font-family: var(--mono); font-size: 11px; background: var(--bg-3); padding: 0 3px; border-radius: 3px; }
</style>
