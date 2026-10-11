// Plain-object mirrors of the Go model (internal/model). The generated
// wailsjs classes carry helper methods, so we use these for app state.

export type Kind = 'http' | 'grpc' | 'ws'

export type KV = { key: string; value: string; disabled?: boolean }

export type Auth = {
  type: 'none' | 'inherit' | 'bearer' | 'basic' | 'apikey' | string
  token?: string
  username?: string
  password?: string
  key?: string
  value?: string
  in?: string
}

export type Body = { type: string; content?: string; form?: KV[] }

export type Settings = { timeoutMs?: number; followRedirects?: boolean; insecureSkipVerify?: boolean }

export type GRPCOpts = { service?: string; method?: string; protoFiles?: string[]; importPaths?: string[]; plaintext?: boolean }

export type WSMessage = { name: string; content: string }

export type Request = {
  name: string
  kind: Kind
  method?: string
  url: string
  params?: KV[]
  headers?: KV[]
  auth?: Auth
  body?: Body
  grpc?: GRPCOpts
  ws?: { subprotocols?: string[]; messages?: WSMessage[] }
  settings?: Settings
  docs?: string
}

export type Variable = { key: string; value: string; secret?: boolean; disabled?: boolean }
export type Environment = { name: string; variables: Variable[] }
export type Folder = { headers?: KV[]; auth?: Auth }

export type HttpResponse = {
  status: number
  statusText: string
  proto: string
  headers: KV[] | null
  contentType: string
  body: string
  bodyBase64: boolean
  size: number
  truncated: boolean
  durationMs: number
  url: string
  method: string
  requestHeaders: KV[] | null
}

export type RunResult = { response: HttpResponse; missing: string[] | null }
export type Preview = { url: string; missing: string[] | null; error: string }

export type AppState = {
  workspace: { root: string; name: string } | null
  environments: string[] | null
  activeEnv: string
  recent: string[] | null
}

export type ScratchTab = { id: string; request: Request }

export type StreamEvent = {
  seq?: number
  session?: string // set by the UI: which connection/call this belongs to
  type: string
  data?: string
  code?: string | number
  binary?: boolean
  metadata?: KV[]
  durationMs?: number
  time: string
}

export type MethodInfo = {
  name: string
  fullName: string
  inputType: string
  outputType: string
  clientStreaming: boolean
  serverStreaming: boolean
  template: string
}
export type ServiceInfo = { name: string; methods: MethodInfo[] }

export type Capture = {
  id: string
  time: string
  method: string
  path: string
  query: string
  headers: { key: string; value: string }[] | null
  body: string
  bodyBase64: boolean
  size: number
  remoteAddr: string
}

export type WebhookStatus = { running: boolean; addr: string }
