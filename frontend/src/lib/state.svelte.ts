import { api, errMsg, newId } from './api'
import { promptFields, promptText, toast } from './dialog.svelte'
import type { AppState, Folder, Request, RunResult, ScratchTab, ServiceInfo, StreamEvent } from './types'
import type { TreeNode } from './tree'
import { folderPaths } from './tree'

export type RequestTab = {
  id: string
  type: 'request'
  source: 'workspace' | 'scratch'
  path: string // workspace path, '' for scratch
  scratchId: string
  request: Request
  saved: string // JSON snapshot for dirty tracking
  // HTTP
  running: boolean
  runId: string
  result: RunResult | null
  error: string
  // WS / gRPC
  log: StreamEvent[]
  sessionId: string
  connected: boolean
  services: ServiceInfo[] | null
  unlisten: (() => void) | null
}

export type FolderTab = { id: string; type: 'folder'; path: string; folder: Folder; saved: string }
export type WebhooksTab = { id: string; type: 'webhooks' }
export type Tab = RequestTab | FolderTab | WebhooksTab

function blankRuntime() {
  return {
    running: false, runId: '', result: null, error: '',
    log: [], sessionId: '', connected: false, services: null, unlisten: null,
  }
}

export function normalize(r: Request): Request {
  r.params ??= []
  r.headers ??= []
  r.kind ??= 'http'
  if (r.kind === 'grpc') {
    r.grpc ??= {}
    r.body ??= { type: 'json', content: '{}' }
  }
  if (r.kind === 'ws') r.ws ??= {}
  return r
}

function snapshot(v: unknown): string {
  return JSON.stringify(v)
}

class AppStore {
  state = $state<AppState>({ workspace: null, environments: [], activeEnv: '', recent: [] })
  tree = $state<TreeNode | null>(null)
  scratch = $state<ScratchTab[]>([])
  tabs = $state<Tab[]>([])
  activeId = $state('')
  envEditor = $state(false)
  importOpen = $state(false)

  active = $derived(this.tabs.find((t) => t.id === this.activeId) ?? null)
  folders = $derived(this.tree ? folderPaths(this.tree) : [''])

  async init() {
    await this.refreshState()
    await this.refreshScratch()
  }

  async refreshState() {
    this.state = await api.getState()
    if (this.state.workspace) {
      try {
        this.tree = await api.getTree()
      } catch (e) {
        toast(errMsg(e), 'error')
      }
    } else {
      this.tree = null
    }
  }

  async refreshScratch() {
    this.scratch = (await api.listScratch()) ?? []
  }

  /** Called when files change on disk (git pull, external editor). Clean
   *  tabs are reloaded; dirty ones are left alone. */
  async onDiskChange() {
    await this.refreshState()
    for (const t of this.tabs) {
      if (t.type === 'request' && t.source === 'workspace' && !isDirty(t)) {
        try {
          const r = normalize(await api.loadRequest(t.path))
          t.request = r
          t.saved = snapshot(r)
        } catch {
          // file deleted or renamed: keep the tab, it becomes "unsaved"
          t.saved = ''
        }
      }
    }
  }

  // ---- workspace ----

  async openWorkspaceDialog() {
    const dir = await api.pickDirectory('Open workspace folder')
    if (dir) await this.openWorkspace(dir)
  }

  async openWorkspace(dir: string) {
    try {
      this.state = await api.openWorkspace(dir)
      this.closeWorkspaceTabs()
      await this.refreshState()
    } catch (e) {
      const msg = errMsg(e)
      if (msg.includes('is not a workspace')) {
        const name = await promptText('Initialise workspace?', `${dir} has no apiclient.yaml. Name for the new workspace:`, dir.split('/').pop() ?? '', 'Create')
        if (name !== null) await this.createWorkspace(dir, name)
      } else toast(msg, 'error')
    }
  }

  async createWorkspaceDialog() {
    const dir = await api.pickDirectory('Choose a folder (e.g. your git repo) for the workspace')
    if (!dir) return
    const name = await promptText('New workspace', 'Name', dir.split('/').pop() ?? '', 'Create')
    if (name !== null) await this.createWorkspace(dir, name)
  }

  async createWorkspace(dir: string, name: string) {
    try {
      this.state = await api.createWorkspace(dir, name)
      this.closeWorkspaceTabs()
      await this.refreshState()
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async closeWorkspace() {
    await api.closeWorkspace()
    this.closeWorkspaceTabs()
    await this.refreshState()
  }

  private closeWorkspaceTabs() {
    for (const t of [...this.tabs]) {
      if (t.type === 'folder' || (t.type === 'request' && t.source === 'workspace')) this.closeTab(t.id, true)
    }
  }

  async setEnv(name: string) {
    await api.setActiveEnvironment(name)
    this.state.activeEnv = name
  }

  // ---- tabs ----

  async openRequest(path: string) {
    const existing = this.tabs.find((t) => t.type === 'request' && t.source === 'workspace' && t.path === path)
    if (existing) {
      this.activeId = existing.id
      return
    }
    try {
      const r = normalize(await api.loadRequest(path))
      const tab: RequestTab = {
        id: newId(), type: 'request', source: 'workspace', path, scratchId: '',
        request: r, saved: snapshot(r), ...blankRuntime(),
      }
      this.tabs.push(tab)
      this.activeId = tab.id
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  openScratch(s: ScratchTab) {
    const existing = this.tabs.find((t) => t.type === 'request' && t.scratchId === s.id)
    if (existing) {
      this.activeId = existing.id
      return
    }
    const r = normalize(s.request)
    const tab: RequestTab = {
      id: newId(), type: 'request', source: 'scratch', path: '', scratchId: s.id,
      request: r, saved: snapshot(r), ...blankRuntime(),
    }
    this.tabs.push(tab)
    this.activeId = tab.id
  }

  async newScratch(kind: 'http' | 'grpc' | 'ws' = 'http', request?: Request) {
    try {
      const s = await api.createScratch(kind)
      if (request) {
        s.request = request
        await api.saveScratch(s)
      }
      this.scratch.push(s)
      this.openScratch(s)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async deleteScratch(id: string) {
    const tab = this.tabs.find((t) => t.type === 'request' && t.scratchId === id)
    if (tab) this.closeTab(tab.id, true)
    await api.deleteScratch(id)
    this.scratch = this.scratch.filter((s) => s.id !== id)
  }

  openWebhooks() {
    const existing = this.tabs.find((t) => t.type === 'webhooks')
    if (existing) {
      this.activeId = existing.id
      return
    }
    const tab: WebhooksTab = { id: newId(), type: 'webhooks' }
    this.tabs.push(tab)
    this.activeId = tab.id
  }

  async openFolderSettings(path: string) {
    const existing = this.tabs.find((t) => t.type === 'folder' && t.path === path)
    if (existing) {
      this.activeId = existing.id
      return
    }
    try {
      const f = await api.loadFolder(path)
      f.headers ??= []
      const tab: FolderTab = { id: newId(), type: 'folder', path, folder: f, saved: snapshot(f) }
      this.tabs.push(tab)
      this.activeId = tab.id
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  closeTab(id: string, force = false) {
    const i = this.tabs.findIndex((t) => t.id === id)
    if (i < 0) return
    const t = this.tabs[i]
    if (!force && t.type === 'request' && t.source === 'workspace' && isDirty(t)) {
      promptFields(`Close "${t.request.name}"?`, [], 'Discard changes').then((r) => {
        if (r) this.closeTab(id, true)
      })
      return
    }
    if (t.type === 'request') stopSession(t)
    this.tabs.splice(i, 1)
    if (this.activeId === id) this.activeId = this.tabs[Math.min(i, this.tabs.length - 1)]?.id ?? ''
  }

  async save(tab: Tab | null = this.active) {
    if (!tab) return
    try {
      if (tab.type === 'request') {
        if (tab.source === 'workspace') {
          await api.saveRequest(tab.path, $state.snapshot(tab.request) as Request)
          tab.saved = snapshot(tab.request)
          await this.refreshState()
        } else {
          await this.promote(tab)
        }
      } else if (tab.type === 'folder') {
        await api.saveFolder(tab.path, $state.snapshot(tab.folder) as Folder)
        tab.saved = snapshot(tab.folder)
      }
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  /** Save a scratch request into the workspace. */
  async promote(tab: RequestTab) {
    if (!this.state.workspace) {
      toast('Open or create a workspace first to save this request into a collection.', 'error')
      return
    }
    const r = await promptFields('Save to workspace', [
      { label: 'Name', value: tab.request.name === 'Untitled' ? '' : tab.request.name },
      { label: 'Folder', value: '', options: this.folders.map((f) => ({ value: f, label: f || '/ (root)' })) },
    ], 'Save')
    if (!r) return
    tab.request.name = r[0] || 'Untitled'
    await api.saveScratch({ id: tab.scratchId, request: $state.snapshot(tab.request) as Request })
    const path = await api.promoteScratch(tab.scratchId, r[1])
    this.scratch = this.scratch.filter((s) => s.id !== tab.scratchId)
    tab.source = 'workspace'
    tab.path = path
    tab.scratchId = ''
    tab.saved = snapshot(tab.request)
    await this.refreshState()
    toast(`Saved to ${path}`)
  }

  async persistScratch(tab: RequestTab) {
    if (tab.source !== 'scratch') return
    const cur = snapshot(tab.request)
    if (cur === tab.saved) return
    try {
      await api.saveScratch({ id: tab.scratchId, request: $state.snapshot(tab.request) as Request })
      tab.saved = cur
      const s = this.scratch.find((x) => x.id === tab.scratchId)
      if (s) s.request = $state.snapshot(tab.request) as Request
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  // ---- tree actions ----

  async newRequest(folder: string, kind: 'http' | 'grpc' | 'ws') {
    const label = { http: 'HTTP request', grpc: 'gRPC call', ws: 'WebSocket' }[kind]
    const name = await promptText(`New ${label}`, 'Name', '', 'Create')
    if (!name) return
    try {
      const path = await api.createRequest(folder, name, kind)
      await this.refreshState()
      await this.openRequest(path)
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  async newFolder(parent: string) {
    const name = await promptText('New folder', 'Name', '', 'Create')
    if (!name) return
    try {
      await api.createFolder(parent, name)
      await this.refreshState()
    } catch (e) {
      toast(errMsg(e), 'error')
    }
  }

  /** Update open tabs after a move/rename of from -> to. */
  retarget(from: string, to: string) {
    for (const t of this.tabs) {
      if ((t.type === 'request' && t.source === 'workspace') || t.type === 'folder') {
        if (t.path === from) t.path = to
        else if (t.path.startsWith(from + '/')) t.path = to + t.path.slice(from.length)
      }
    }
  }

  closeUnder(path: string) {
    for (const t of [...this.tabs]) {
      if ((t.type === 'request' && t.source === 'workspace') || t.type === 'folder') {
        if (t.path === path || t.path.startsWith(path + '/')) this.closeTab(t.id, true)
      }
    }
  }
}

export function isDirty(t: Tab): boolean {
  if (t.type === 'request') return snapshot(t.request) !== t.saved
  if (t.type === 'folder') return snapshot(t.folder) !== t.saved
  return false
}

export function stopSession(t: RequestTab) {
  t.unlisten?.()
  t.unlisten = null
  if (t.runId) api.cancelRun(t.runId)
  if (t.sessionId) {
    if (t.request.kind === 'ws') api.wsClose(t.sessionId)
    if (t.request.kind === 'grpc') api.grpcCancel(t.sessionId)
  }
  t.connected = false
}

export const app = new AppStore()
