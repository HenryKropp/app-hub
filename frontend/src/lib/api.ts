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
}

export type AppDetail = {
  id: number
  folder_path: string
  folder_display: string
  owner_id: number
  backup_id: number | null
  manifest: Manifest
  updated_at: string
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
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
  if (!res.ok) throw new ApiError(res.status, data?.error || "Não foi possível completar a ação.")
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
}
