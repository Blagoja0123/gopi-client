// Typed facade over the generated Wails bindings, using plain-object types.
import * as G from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type {
  AppState, Capture, Environment, Folder, HttpResponse, Preview, Request, RunResult,
  ScratchTab, ServiceInfo, StreamEvent, Variable, WebhookStatus,
} from './types'
import type { TreeNode } from './tree'

const A = G as any

export const api = {
  getState: (): Promise<AppState> => A.GetState(),
  pickDirectory: (title: string): Promise<string> => A.PickDirectory(title),
  pickFile: (title: string, pattern: string): Promise<string> => A.PickFile(title, pattern),
  openWorkspace: (root: string): Promise<AppState> => A.OpenWorkspace(root),
  createWorkspace: (root: string, name: string): Promise<AppState> => A.CreateWorkspace(root, name),
  closeWorkspace: (): Promise<void> => A.CloseWorkspace(),

  getTree: (): Promise<TreeNode> => A.GetTree(),
  loadRequest: (path: string): Promise<Request> => A.LoadRequest(path),
  saveRequest: (path: string, r: Request): Promise<void> => A.SaveRequest(path, r),
  createRequest: (folder: string, name: string, kind: string): Promise<string> => A.CreateRequest(folder, name, kind),
  createFolder: (parent: string, name: string): Promise<string> => A.CreateFolder(parent, name),
  deletePath: (path: string): Promise<void> => A.DeletePath(path),
  movePath: (from: string, to: string): Promise<void> => A.MovePath(from, to),
  loadFolder: (path: string): Promise<Folder> => A.LoadFolder(path),
  saveFolder: (path: string, f: Folder): Promise<void> => A.SaveFolder(path, f),

  getGlobals: (): Promise<Variable[] | null> => A.GetGlobals(),
  saveGlobals: (v: Variable[]): Promise<void> => A.SaveGlobals(v),
  getEnvironment: (name: string): Promise<Environment> => A.GetEnvironment(name),
  saveEnvironment: (env: Environment): Promise<void> => A.SaveEnvironment(env),
  deleteEnvironment: (name: string): Promise<void> => A.DeleteEnvironment(name),
  setActiveEnvironment: (name: string): Promise<void> => A.SetActiveEnvironment(name),
  clearCookies: (): Promise<void> => A.ClearCookies(),

  listScratch: (): Promise<ScratchTab[] | null> => A.ListScratch(),
  createScratch: (kind: string): Promise<ScratchTab> => A.CreateScratch(kind),
  saveScratch: (t: ScratchTab): Promise<void> => A.SaveScratch(t),
  deleteScratch: (id: string): Promise<void> => A.DeleteScratch(id),
  promoteScratch: (id: string, folder: string): Promise<string> => A.PromoteScratch(id, folder),

  preview: (path: string, r: Request): Promise<Preview> => A.PreviewRequest(path, r),
  sendHTTP: (runId: string, path: string, r: Request): Promise<RunResult> => A.SendHTTP(runId, path, r),
  cancelRun: (runId: string): Promise<void> => A.CancelRun(runId),
  exportCurl: (path: string, r: Request, resolved: boolean): Promise<string> => A.ExportCurl(path, r, resolved),
  importCurl: (cmd: string): Promise<Request> => A.ImportCurl(cmd),
  importOpenAPIFile: (p: string): Promise<string> => A.ImportOpenAPIFile(p),
  importOpenAPIURL: (u: string): Promise<string> => A.ImportOpenAPIURL(u),

  wsConnect: (id: string, path: string, r: Request): Promise<void> => A.WSConnect(id, path, r),
  wsSend: (id: string, text: string): Promise<void> => A.WSSend(id, text),
  wsClose: (id: string): Promise<void> => A.WSClose(id),

  grpcDescribe: (path: string, r: Request): Promise<ServiceInfo[] | null> => A.GRPCDescribe(path, r),
  grpcStart: (id: string, path: string, r: Request): Promise<void> => A.GRPCStart(id, path, r),
  grpcSend: (id: string, body: string): Promise<void> => A.GRPCSend(id, body),
  grpcCloseSend: (id: string): Promise<void> => A.GRPCCloseSend(id),
  grpcCancel: (id: string): Promise<void> => A.GRPCCancel(id),

  webhookStart: (addr: string): Promise<WebhookStatus> => A.WebhookStart(addr),
  webhookStop: (): Promise<void> => A.WebhookStop(),
  webhookStatus: (): Promise<WebhookStatus> => A.WebhookGetStatus(),
  webhookCaptures: (): Promise<Capture[] | null> => A.WebhookCaptures(),
  webhookClear: (): Promise<void> => A.WebhookClear(),
  webhookSetResponse: (r: { status: number; contentType: string; body: string }): Promise<void> => A.WebhookSetResponse(r),
  webhookReplay: (id: string, target: string): Promise<HttpResponse> => A.WebhookReplay(id, target),
  webhookSaveCapture: (id: string, folder: string, base: string): Promise<string> => A.WebhookSaveCapture(id, folder, base),
  signPayload: (preset: string, secret: string, payload: string): Promise<{ header: string; value: string }> =>
    A.SignPayload(preset, secret, payload),
  verifySignature: (preset: string, secret: string, id: string): Promise<boolean> => A.VerifySignature(preset, secret, id),
}

export function on<T = StreamEvent>(event: string, cb: (data: T) => void): () => void {
  return EventsOn(event, cb as (...d: any) => void)
}

export function newId(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36)
}

export function errMsg(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return JSON.stringify(e)
}
