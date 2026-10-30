export type Person = {
  id: number; name: string; email: string
  role_id: number; role_name: string; permissions: string[]
  leads: number[]; areas: number[]; sees_others_runs: boolean
}
export type PersonRow = { id: number; name: string; email: string; role_id: number; role_name: string; area_ids: number[] }
export type Can = { view: boolean; edit: boolean; restore: boolean; tech: boolean; runs: boolean; access: boolean; grants: boolean }
export type RoleInfo = { id: number; key: string; name: string; permissions: string[]; people_count: number }
export type NameRef = { id: number; name: string }
export type Area = { id: number; name: string; app_count: number; leaders: NameRef[]; members: NameRef[] }
export type Grant = { person_id: number; name: string; permissions: string[] }
export type AppAccess = { restricted: boolean; areas: NameRef[]; people: NameRef[]; grants: Grant[]; area_grants: Grant[] }
export type AppLink = { id: number; slug: string; name: string; visible: boolean }

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
  flag?: string
}

export type Output = { label: string; pattern: string }
export type SecretRef = { name: string; file: string; line: number; hint?: string }
export type PersonRef = { name: string; email: string }

export type EnvVar = { name: string; label: string; secret?: boolean; required?: boolean; default?: string }
export type SharedSpec = {
  enabled: boolean; port?: number; health_path?: string; database?: "" | "postgres"
  always_on?: boolean; idle_minutes?: number; memory_mb?: number; cpus?: number
  python?: string; node?: string; command?: string; env?: EnvVar[] | null
}

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
  shared?: SharedSpec | null
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

export type SharedStatus = "draft" | "starting" | "running" | "sleeping" | "stopped" | "crashed"
export type SharedBrief = { status: SharedStatus; url: string; slug: string }
export type SharedState = {
  status: SharedStatus; status_message: string; stopped_by: string; always_on: boolean; idle_minutes: number
  memory_mb: number; cpus: number; wants_db: boolean; url: string; acks: string[]; has_previous: boolean
}
export type Deployment = {
  id: number; kind: "test" | "live"; status: "building" | "ready" | "live" | "failed" | "retired"
  version_number: number | null; error_message: string; created_by_name: string
  created_at: string; finished_at: string | null; expires_at: string | null
  build_log?: string; port?: number // only for whoever sees the technical part
}
export type SharedInfo = {
  shared: SharedState | null; test_url?: string; test_open?: boolean
  live_version?: number; latest_version?: number; deployments?: Deployment[]
}
export type ReadinessItem = { key: string; status: "ok" | "auto" | "missing" | "warn"; title: string; detail: string; claude_text: string; acked: boolean }
export type Readiness = { ready: boolean; recipe: { kind: string; dockerfile: string; port: number; command: string; needs_confirm: boolean; requirements: string[] | null }; wants_db: boolean; suggest_env: EnvVar[] | null; items: ReadinessItem[] }
export type SharedSettings = {
  enabled: boolean; always_on: boolean; idle_minutes: number; memory_mb: number; cpus: number; port: number
  health_path: string; database: "" | "postgres"; command: string; env: EnvVar[]; acks: string[]; updated_at: string
}
export type ConfigItem = EnvVar & { has_value: boolean; value: string }
export type SharedBackup = { id: number; kind: "daily" | "manual" | "pre_deploy" | "pre_restore"; size_bytes: number; created_at: string; expires_at: string | null; created_by_name: string; version_number: number | null }
export type UsagePoint = { at: string; mem_mb: number; cpu: number }
export type ServerApp = {
  app_id: number; name: string; slug: string; status: SharedStatus; always_on: boolean; memory_mb: number; cpus: number; idle_minutes: number
  hours_on: number; avg_mem_mb: number; max_mem_mb: number; avg_cpu: number; db_bytes: number; backup_bytes: number; cost_cents: number
}
export type ServerInfo = {
  memory_total: number; memory_used: number; disk_total: number; disk_used: number; disk_warning: boolean; engine_ok: boolean
  monthly_cost_cents: number; hub_cents: number; period_start: string; apps: ServerApp[]
}

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
  area_id: number
  restricted: boolean
  can: Can
  shared: SharedBrief | null
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
  area_id: number
  restricted: boolean
  can: Can
  shared: SharedBrief | null
  /** Only in the answer of PUT /shared: the running app must start again for the change to count. */
  needs_restart?: boolean
  /** Only in the answer of PUT /shared: a new port counts only after a new publish. */
  needs_publish?: boolean
  depends_on_apps?: AppLink[]
  used_by?: AppLink[]
}

export type UpdateState = "" | "updating" | "updated" | "gave_up"
export type Computer = {
  id: number; name: string; agent_version: string; last_seen_at: string | null; connected: boolean; outdated: boolean
  update_state: UpdateState; update_version?: string; updated_at?: string
}
export type ComputersInfo = { computers: Computer[]; expected_version: string; agent_available: boolean }
export type Pairing = { code: string; computer_name: string; expired: boolean; approved: boolean }
export type RunStatus = "waiting" | "running" | "done" | "no_files" | "error" | "interrupted" | "stopped"
export type RunFile = { id: number; path: string; size: number; modified: string; change: "new" | "changed" }
export type RunGroup = { index: number; label: string; files: RunFile[] }
export type Run = {
  id: number; app_id: number; app_name: string; person_name: string; computer_name: string; output_folder: string
  status: RunStatus; exit_code: number | null; error_message: string; app_version: string; version_number: number | null
  created_at: string; started_at: string | null; finished_at: string | null; groups: RunGroup[]
  // Only for the person who ran it, while it is open and their Agente can stop it.
  can_stop: boolean
}
export type ActiveRun = {
  id: number; app_id: number; app_name: string; app_folder_display: string; status: RunStatus; error_message: string
  output_folder: string; file_count: number; created_at: string; finished_at: string | null; can_stop: boolean
}
export type RunListItem = {
  id: number; app_id: number; app_name: string; status: RunStatus; output_folder: string; person_id: number; person_name: string
  created_at: string; started_at: string | null; finished_at: string | null; file_count: number
}
export type RunList = { items: RunListItem[]; more: boolean }
export type AppCheck = { folder_readable: boolean; version: { number: number; created_at: string } | null }
export type Onboarding = {
  company_name: string
  can_manage: boolean
  agent_available: boolean
  computer_connected: boolean
  has_apps: boolean
  has_successful_run: boolean
  has_team: boolean
}
export type Workspace = { slug: string; name: string; base_path: string; can_manage: boolean; local_execution_is_sandboxed: false; filesystem_registration_enabled?: boolean }
export type WorkspaceCreated = { slug: string; name: string; url: string }
export type EntryCompany = { name: string; slug: string; base_path: string }
export type EntryCompanyCreated = { url: string; name: string; slug: string }
export type EntryInvitationPreview = { email: string; company_name: string }
export type MembershipInvitation = { id: number; email: string; role_id: number; role_name: string; expires_at: string; created_at: string }
export type MembershipRequest = { id: number; name: string; email: string; created_at: string }
export type Membership = { invitations: MembershipInvitation[]; requests: MembershipRequest[] }
export type SecuritySettings = { retention_days: number; require_app_approval: boolean; block_embedded_secrets: boolean; local_execution_enabled: boolean }
export type SecurityApp = { id: number; name: string; classification: string; purpose: string; external_domains: string[]; approved: boolean; suspended: boolean }
export type SecurityInfo = {
  can_manage: boolean
  settings: SecuritySettings
  limits: { sandboxed: false; vault_available: false; external_marketplace_enabled: false }
  apps: SecurityApp[]
}
export type AuditInfo = { events: { id: number; actor_id: number | null; action: string; route: string; status: number; created_at: string }[] }
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

// The browser's own errors ("Failed to fetch") are English and say nothing; these say what to do.
export const OFFLINE = "Não consegui falar com o Ziv. Confira se esta página ainda está aberta no endereço certo e tente novamente."
const DOWN = "O Ziv não respondeu. Espere um pouco e tente de novo."

export const workspaceBasePath = (pathname = window.location.pathname) =>
  pathname.match(/^\/w\/[a-z0-9-]+(?=\/|$)/)?.[0] ?? ""
export const workspaceUrl = (path: string) => `${workspaceBasePath()}${path.startsWith("/") ? path : `/${path}`}`

async function requestAt<T>(method: string, url: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, {
      method,
      credentials: "same-origin",
      headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError(0, OFFLINE)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text().catch(() => "")
  if (res.ok && text === "") return undefined as T
  const data = (() => { try { return text ? JSON.parse(text) : {} } catch { return {} } })()
  if (!res.ok) throw new ApiError(res.status, data?.error || (res.status >= 500 ? DOWN : "Não foi possível completar a ação."), data)
  return data as T
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  return requestAt<T>(method, workspaceUrl(path), body)
}

export const api = {
  workspace: () => request<Workspace>("GET", "/api/workspace"),
  entryCompany: () => request<EntryCompany>("GET", "/api/entry/company"),
  entryCompanyBySlug: (slug: string) =>
    requestAt<EntryCompany>("GET", `/w/${encodeURIComponent(slug)}/api/entry/company`),
  createEntryCompany: (b: { company_name: string; slug: string; name: string; email: string; password: string }) =>
    requestAt<EntryCompanyCreated>("POST", "/api/entry/companies", b),
  requestCompanyAccess: (b: { name: string; email: string }) =>
    request<{ message: string }>("POST", "/api/entry/requests", b),
  previewInvitation: (token: string) =>
    request<EntryInvitationPreview>("POST", "/api/entry/invitations/preview", { token }),
  acceptInvitation: (b: { token: string; name: string; password: string }) =>
    request<{ url: string }>("POST", "/api/entry/invitations/accept", b),
  membership: () => request<Membership>("GET", "/api/membership"),
  createMembershipInvitation: (b: { email: string; role_id: number }) =>
    request<{ url: string }>("POST", "/api/membership/invitations", b),
  revokeMembershipInvitation: (id: number) =>
    request<{ ok: true }>("POST", `/api/membership/invitations/${id}/revoke`),
  approveMembershipRequest: (id: number, role_id: number) =>
    request<{ url: string }>("POST", `/api/membership/requests/${id}/approve`, { role_id }),
  rejectMembershipRequest: (id: number) =>
    request<{ ok: true }>("POST", `/api/membership/requests/${id}/reject`, {}),
  createWorkspace: (b: { name: string; slug: string; password: string }) =>
    request<WorkspaceCreated>("POST", "/api/workspaces", b),
  security: () => request<SecurityInfo>("GET", "/api/security"),
  saveSecuritySettings: (b: SecuritySettings) => request<SecuritySettings>("PUT", "/api/security/settings", b),
  saveSecurityApp: (id: number, b: { classification: "internal" | "confidential" | "restricted"; purpose: string; external_domains: string[]; approved: boolean; suspended: boolean }) =>
    request<SecurityApp>("PUT", `/api/security/apps/${id}`, b),
  securityAudit: () => request<AuditInfo>("GET", "/api/security/audit"),
  securityExport: () => request<unknown>("GET", "/api/security/export"),
  purgeRuns: () => request<{ deleted_runs: number }>("POST", "/api/security/purge", { confirm: true }),
  status: () => request<{ needs_setup: boolean; person: Person | null }>("GET", "/api/auth/status"),
  onboarding: () => request<Onboarding>("GET", "/api/onboarding"),
  saveCompanyName: (company_name: string) => request<Onboarding>("PUT", "/api/onboarding", { company_name }),
  setup: (b: { name: string; email: string; password: string }) => request<Person>("POST", "/api/auth/setup", b),
  login: (b: { email: string; password: string }) => request<Person>("POST", "/api/auth/login", b),
  logout: () => request<void>("POST", "/api/auth/logout"),
  people: () => request<PersonRow[]>("GET", "/api/people"),
  createPerson: (b: { name: string; email: string; password: string; role_id: number }) => request<PersonRow>("POST", "/api/people", b),
  scan: (path: string, mode: "batch" | "single") => request<ScanResult>("POST", "/api/scan", { path, mode }),
  apps: () => request<AppSummary[]>("GET", "/api/apps"),
  app: (id: number | string) => request<AppDetail>("GET", `/api/apps/${id}`),
  saveApp: (b: { folder_path: string; owner_id: number; backup_id: number | null; manifest: Manifest }) =>
    request<AppDetail>("POST", "/api/apps", b),
  importScript: (b: { manifest: Manifest; files: Record<string, { name: string; content_base64: string }>; confirm_secrets?: boolean }) =>
    request<{ id: number }>("POST", "/api/scripts/import", b),
  updateApp: (id: number, b: { owner_id: number; backup_id: number | null; manifest: Manifest; updated_at: string }) =>
    request<AppDetail>("PUT", `/api/apps/${id}`, b),
  setPersonRole: (id: number, role_id: number) => request<PersonRow>("PUT", `/api/people/${id}`, { role_id }),
  roles: () => request<RoleInfo[]>("GET", "/api/roles"),
  createRole: (b: { name: string; permissions: string[] }) => request<RoleInfo>("POST", "/api/roles", b),
  updateRole: (id: number, b: { name: string; permissions: string[] }) => request<RoleInfo>("PUT", `/api/roles/${id}`, b),
  deleteRole: (id: number) => request<void>("DELETE", `/api/roles/${id}`),
  areas: () => request<Area[]>("GET", "/api/areas"),
  createArea: (name: string) => request<Area>("POST", "/api/areas", { name }),
  renameArea: (id: number, name: string) => request<Area>("PUT", `/api/areas/${id}`, { name }),
  deleteArea: (id: number) => request<void>("DELETE", `/api/areas/${id}`),
  setAreaLeaders: (id: number, person_ids: number[]) => request<void>("PUT", `/api/areas/${id}/leaders`, { person_ids }),
  setAreaMembers: (id: number, person_ids: number[]) => request<void>("PUT", `/api/areas/${id}/members`, { person_ids }),
  appAccess: (id: number) => request<AppAccess>("GET", `/api/apps/${id}/access`),
  setAppAccess: (id: number, b: { restricted: boolean; area_ids: number[]; person_ids: number[] }) =>
    request<AppAccess>("PUT", `/api/apps/${id}/access`, b),
  setGrants: (id: number, b: { person_id: number; permissions: string[]; whole_area: boolean }) =>
    request<AppAccess>("PUT", `/api/apps/${id}/grants`, b),
  computers: () => request<ComputersInfo>("GET", "/api/computers/me"),
  disconnect: (id: number) => request<void>("DELETE", `/api/computers/${id}`),
  pairing: (code: string) => request<Pairing>("GET", `/api/pairings/${encodeURIComponent(code)}`),
  approvePairing: (code: string) => request<void>("POST", `/api/pairings/${encodeURIComponent(code)}/approve`),
  chooseFolder: (initial: string) => request<{ folder: string; canceled: boolean }>("POST", "/api/choose-folder", { initial }),
  startRun: (
    appId: number,
    output_folder: string,
    remember?: boolean,
    inputs?: Record<string, string | string[]>,
    files?: Record<string, { name: string; content_base64: string }>,
  ) => {
    const body: { output_folder: string; remember?: boolean; inputs?: Record<string, string | string[]>; files?: Record<string, { name: string; content_base64: string }> } = { output_folder }
    if (remember !== undefined) body.remember = remember
    if (inputs && Object.keys(inputs).length) body.inputs = inputs
    if (files && Object.keys(files).length) body.files = files
    return request<ActiveRun>("POST", `/api/apps/${appId}/runs`, body)
  },
  checkApp: (id: number | string) => request<AppCheck>("POST", `/api/apps/${id}/check`),
  activeRuns: () => request<ActiveRun[]>("GET", "/api/runs/active"),
  dismissRun: (id: number) => request<void>("POST", `/api/runs/${id}/dismiss`),
  stopRun: (id: number) => request<void>("POST", `/api/runs/${id}/stop`),
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
  shared: {
    get: (id: number) => request<SharedInfo>("GET", `/api/apps/${id}/shared`),
    check: (id: number) => request<Readiness>("GET", `/api/apps/${id}/shared/check`),
    save: (id: number, s: SharedSettings) => request<AppDetail>("PUT", `/api/apps/${id}/shared`, s),
    test: (id: number) => request<Deployment>("POST", `/api/apps/${id}/shared/test`),
    publish: (id: number) => request<Deployment>("POST", `/api/apps/${id}/shared/publish`),
    deployment: (id: number, dep: number) => request<Deployment>("GET", `/api/apps/${id}/shared/deployments/${dep}`),
    rollback: (id: number, withData: boolean) => request<object>("POST", `/api/apps/${id}/shared/rollback`, { with_data: withData }),
    stop: (id: number) => request<object>("POST", `/api/apps/${id}/shared/stop`),
    start: (id: number) => request<object>("POST", `/api/apps/${id}/shared/start`),
    unshare: (id: number) => request<object>("POST", `/api/apps/${id}/shared/unshare`),
    logs: (id: number) => request<{ text: string }>("GET", `/api/apps/${id}/shared/logs`),
    usage: (id: number) => request<UsagePoint[]>("GET", `/api/apps/${id}/shared/usage`),
    config: (id: number) => request<ConfigItem[]>("GET", `/api/apps/${id}/shared/config`),
    setConfig: (id: number, name: string, value: string) => request<{ needs_restart: boolean }>("PUT", `/api/apps/${id}/shared/config`, { name, value }),
    backups: (id: number) => request<SharedBackup[]>("GET", `/api/apps/${id}/shared/backups`),
    backup: (id: number) => request<SharedBackup>("POST", `/api/apps/${id}/shared/backups`),
    restore: (id: number, bid: number) => request<object>("POST", `/api/apps/${id}/shared/backups/${bid}/restore`),
  },
  server: {
    get: () => request<ServerInfo>("GET", "/api/server"),
    setCost: (cents: number) => request<object>("PUT", "/api/server/cost", { monthly_cost_cents: cents }),
    setApp: (id: number, l: { memory_mb: number; cpus: number; always_on: boolean; idle_minutes: number }) =>
      request<{ needs_restart: boolean }>("PUT", `/api/server/apps/${id}`, l),
    stop: (id: number) => request<object>("POST", `/api/server/apps/${id}/stop`),
    start: (id: number) => request<object>("POST", `/api/server/apps/${id}/start`),
  },
}

export const AGENT_DOWNLOAD_URL = workspaceUrl("/api/agent/download")
export const fileUrl = (runId: number, fileId: number) => workspaceUrl(`/api/runs/${runId}/files/${fileId}`)
export const groupZipUrl = (runId: number, index: number) => workspaceUrl(`/api/runs/${runId}/groups/${index}/zip`)
export const selectedUrl = (runId: number, fileIds: number[]) => workspaceUrl(`/api/runs/${runId}/selected?files=${fileIds.join(",")}`)
export const backupDownloadUrl = (id: number, bid: number) => workspaceUrl(`/api/apps/${id}/shared/backups/${bid}/download`)
export const openSharedUrl = (slug: string, test = false) =>
  workspaceUrl(`/api/shared/authorize?app=${encodeURIComponent(slug)}${test ? "&teste=1" : ""}&volta=%2F`)
