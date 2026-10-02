export type Role = "admin" | "criador" | "usuario"

export type Person = { id: number; name: string; email: string; role: Role }

export type Kind = "script" | "web" | "desktop" | "scheduled" | "executable" | "reference" | "system"

export type Input = {
  id: string
  label: string
  type: "file" | "folder" | "text" | "number" | "date" | "choice"
  source: "fixed_file" | "dialog" | "prompt" | "argument" | "window"
  path?: string
  options?: string[] | null
  multiple?: boolean
  required: boolean
}

export type Output = { label: string; pattern: string }
export type SecretRef = { name: string; file: string; line: number; hint?: string }
export type PersonRef = { name: string; email: string }

export type Manifest = {
  manifest_version: number
  id: string
  name: string
  summary: string
  kind: Kind
  language: string
  entrypoint: string
  run: { command: string; url: string }
  runtime: { language: string; dependencies_file: string; dependencies: string[] | null }
  inputs: Input[] | null
  outputs: Output[] | null
  secrets: SecretRef[] | null
  schedule: string
  area: string
  owner: PersonRef | null
  backup: PersonRef | null
  tags: string[] | null
  depends_on?: string[] | null
  build?: { source: string; command: string } | null
  notes: string[] | null
}

export type Question = { field: string; text: string }

export type ScanItem = {
  folder_path: string
  folder_name: string
  folder_display: string
  manifest: Manifest
  findings: string[] | null
  questions: Question[] | null
  entrypoint_options: string[] | null
  secrets: SecretRef[] | null
  file_count: number
  truncated: boolean
  has_manifest: boolean
  suggest_skip: boolean
  registered_app_id: number
  similar: { name: string; app_id?: number; folder_name?: string }[]
}

export type ScanResult = { root_display: string; items: ScanItem[] }

export type AppSummary = {
  id: number
  slug: string
  name: string
  summary: string
  kind: Kind
  language: string
  area: string
  folder_path: string
  folder_display: string
  owner_id: number
  owner_name: string
  backup_id: number | null
  updated_at: string
  can_run: boolean
  remembered_folder: string
}

export type AppDetail = {
  id: number
  folder_path: string
  folder_display: string
  owner_id: number
  backup_id: number | null
  manifest: Manifest
  updated_at: string
  can_run: boolean
  version: { number: number; created_at: string } | null
  folder_readable: boolean
  remembered_folder: string
}

export type UpdateState = "" | "updating" | "updated" | "gave_up"
export type Computer = {
  id: number; name: string; agent_version: string; last_seen_at: string | null; connected: boolean; outdated: boolean
  update_state: UpdateState; update_version?: string; updated_at?: string
}
export type ComputersInfo = { computers: Computer[]; expected_version: string; agent_available: boolean }
export type Pairing = { code: string; computer_name: string; expired: boolean; approved: boolean }
export type RunStatus = "waiting" | "running" | "done" | "no_files" | "error" | "interrupted"
export type RunFile = { id: number; path: string; size: number; modified: string; change: "new" | "changed" }
export type RunGroup = { index: number; label: string; files: RunFile[] }
export type Run = {
  id: number; app_id: number; app_name: string; person_name: string; computer_name: string; output_folder: string
  status: RunStatus; exit_code: number | null; error_message: string; app_version: string; version_number: number | null
  created_at: string; started_at: string | null; finished_at: string | null; groups: RunGroup[]
}
export type ActiveRun = {
  id: number; app_id: number; app_name: string; app_folder_display: string; status: RunStatus; error_message: string
  output_folder: string; file_count: number; created_at: string; finished_at: string | null
}
export type RunListItem = {
  id: number; app_id: number; app_name: string; status: RunStatus; output_folder: string; person_id: number; person_name: string
  created_at: string; started_at: string | null; finished_at: string | null; file_count: number
}
export type RunList = { items: RunListItem[]; more: boolean }
export type AppCheck = { folder_readable: boolean; version: { number: number; created_at: string } | null }
export type RunSummary = { id: number; status: RunStatus; person_name: string; created_at: string; started_at: string | null; finished_at: string | null; file_count: number }

export type VersionSummary = { added: number; removed: number; modified: number; binaries: string[] | null }
export type VersionInfo = {
  number: number; origin: "original" | "folder" | "restore"; restored_from: number | null
  author_name: string; author_probable: boolean; description: string; summary: VersionSummary
  files_changed_at: string | null; created_at: string; current: boolean
}
export type FichaChange = { field: string; label: string; before?: string; after?: string; long?: boolean }
export type HistoryItem = {
  type: "version" | "run" | "ficha"; at: string
  version?: VersionInfo
  run?: { id: number; status: RunStatus; person_name: string; version_number: number | null }
  ficha?: { person_name: string; changes: FichaChange[] }
}
export type HistoryFilter = "all" | "code" | "runs" | "ficha"
export type History = { items: HistoryItem[]; more: boolean; next_before: string }
export type DiffLine = { op: " " | "-" | "+"; text: string; old: number; new: number }
export type FileChange = { path: string; status: "added" | "removed" | "modified"; kind: "code" | "binary"; old_size: number | null; new_size: number | null; hunks: { lines: DiffLine[] }[] }
export type VersionFile = { path: string; size: number; kind: "code" | "binary"; modified: string }
export type RestorePlan = { rewrite: string[]; recreate: string[]; to_trash: string[]; binaries: string[] }
export type RestoreResult = { number: number; done: string[]; failed: { path: string; error: string }[]; error?: string }

export class ApiError extends Error {
  status: number
  body?: unknown
  constructor(status: number, message: string, body?: unknown) {
    super(message)
    this.status = status
    this.body = body
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new ApiError(res.status, data?.error || "Não foi possível completar a ação.", data)
  return data as T
}

export const api = {
  status: () => request<{ needs_setup: boolean; person: Person | null }>("GET", "/api/auth/status"),
  setup: (b: { name: string; email: string; password: string }) => request<Person>("POST", "/api/auth/setup", b),
  login: (b: { email: string; password: string }) => request<Person>("POST", "/api/auth/login", b),
  logout: () => request<void>("POST", "/api/auth/logout"),
  people: () => request<Person[]>("GET", "/api/people"),
  createPerson: (b: { name: string; email: string; password: string; role: Role }) => request<Person>("POST", "/api/people", b),
  scan: (path: string, mode: "batch" | "single") => request<ScanResult>("POST", "/api/scan", { path, mode }),
  apps: () => request<AppSummary[]>("GET", "/api/apps"),
  app: (id: number | string) => request<AppDetail>("GET", `/api/apps/${id}`),
  saveApp: (b: { folder_path: string; owner_id: number; backup_id: number | null; manifest: Manifest }) =>
    request<AppDetail>("POST", "/api/apps", b),
  computers: () => request<ComputersInfo>("GET", "/api/computers/me"),
  disconnect: (id: number) => request<void>("DELETE", `/api/computers/${id}`),
  pairing: (code: string) => request<Pairing>("GET", `/api/pairings/${encodeURIComponent(code)}`),
  approvePairing: (code: string) => request<void>("POST", `/api/pairings/${encodeURIComponent(code)}/approve`),
  chooseFolder: (initial: string) => request<{ folder: string; canceled: boolean }>("POST", "/api/choose-folder", { initial }),
  startRun: (appId: number, output_folder: string, remember?: boolean) =>
    request<Run>("POST", `/api/apps/${appId}/runs`, remember === undefined ? { output_folder } : { output_folder, remember }),
  checkApp: (id: number | string) => request<AppCheck>("POST", `/api/apps/${id}/check`),
  activeRuns: () => request<ActiveRun[]>("GET", "/api/runs/active"),
  dismissRun: (id: number) => request<void>("POST", `/api/runs/${id}/dismiss`),
  runs: (scope: "mine" | "all", before?: number) =>
    request<RunList>("GET", `/api/runs?scope=${scope}${before ? `&before=${before}` : ""}`),
  appRuns: (appId: number | string) => request<RunSummary[]>("GET", `/api/apps/${appId}/runs`),
  history: (appId: number | string, filter: HistoryFilter, before?: string) =>
    request<History>("GET", `/api/apps/${appId}/history?type=${filter}${before ? `&before=${encodeURIComponent(before)}` : ""}`),
  version: (appId: number | string, n: number | string) => request<VersionInfo>("GET", `/api/apps/${appId}/versions/${n}`),
  versionChanges: (appId: number | string, n: number | string) =>
    request<{ version: VersionInfo; files: FileChange[] }>("GET", `/api/apps/${appId}/versions/${n}/changes`),
  versionFiles: (appId: number | string, n: number | string) => request<VersionFile[]>("GET", `/api/apps/${appId}/versions/${n}/files`),
  versionContent: (appId: number | string, n: number | string, path: string) =>
    request<{ path: string; text: string }>("GET", `/api/apps/${appId}/versions/${n}/content?path=${encodeURIComponent(path)}`),
  describeVersion: (appId: number, n: number, b: { description: string; author_id: number | null }) =>
    request<VersionInfo>("PATCH", `/api/apps/${appId}/versions/${n}`, b),
  restorePreview: (appId: number, n: number) => request<{ plan: RestorePlan; current: number }>("GET", `/api/apps/${appId}/versions/${n}/restore-preview`),
  restore: (appId: number, n: number) => request<RestoreResult>("POST", `/api/apps/${appId}/versions/${n}/restore`),
  run: (id: number | string) => request<Run>("GET", `/api/runs/${id}`),
}

export const AGENT_DOWNLOAD_URL = "/api/agent/download"
export const fileUrl = (runId: number, fileId: number) => `/api/runs/${runId}/files/${fileId}`
export const groupZipUrl = (runId: number, index: number) => `/api/runs/${runId}/groups/${index}/zip` 
export const selectedUrl = (runId: number, fileIds: number[]) => `/api/runs/${runId}/selected?files=${fileIds.join(",")}`
