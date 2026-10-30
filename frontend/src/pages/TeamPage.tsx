import { useCallback, useEffect, useState, type FormEvent } from "react"
import { Navigate, useSearchParams } from "react-router-dom"
import { toast } from "sonner"
import { Check, Copy, Link2, MailPlus, RefreshCw, UserCheck, UserX } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, LoadError, NativeSelect, PageHeader } from "@/components/bits"
import { api, workspaceBasePath, type Area, type Membership, type Person, type PersonRow, type RoleInfo } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { PERMISSIONS, canSeeTeam, has, permLabel } from "@/lib/permissions"

type Tab = "pessoas" | "areas" | "cargos" | "acessos"
const TAB_NAMES: Record<Tab, string> = { pessoas: "Pessoas", areas: "Áreas", cargos: "Cargos", acessos: "Acessos" }
const textBtn = "text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
const errText = (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback)
const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

export function TeamPage() {
  const { person: me } = useAuth()
  const [params, setParams] = useSearchParams()
  const [people, setPeople] = useState<PersonRow[]>([])
  const [roles, setRoles] = useState<RoleInfo[]>([])
  const [areas, setAreas] = useState<Area[]>([])
  const [membership, setMembership] = useState<Membership>({ invitations: [], requests: [] })
  const [loadError, setLoadError] = useState("")
  const [loaded, setLoaded] = useState(false)

  const canPeople = has(me, "manage_people")
  const canAreas = has(me, "manage_areas")
  const canRoles = has(me, "manage_roles")
  const leads = me?.leads ?? []

  const load = useCallback(() => {
    const wantPeople = has(me, "manage_people") || has(me, "manage_areas") || (me?.leads.length ?? 0) > 0
    Promise.all([
      wantPeople ? api.people() : Promise.resolve([] as PersonRow[]),
      has(me, "manage_people") || has(me, "manage_roles") ? api.roles() : Promise.resolve([] as RoleInfo[]),
      api.areas(),
      has(me, "manage_roles") ? api.membership() : Promise.resolve({ invitations: [], requests: [] } as Membership),
    ]).then(
      ([p, r, a, m]) => {
        setLoadError("")
        setPeople(p)
        setRoles(r)
        setAreas(a)
        setMembership(m)
        setLoaded(true)
      },
      (e: unknown) => setLoadError(errText(e, "Não foi possível carregar a equipe.")),
    )
  }, [me])

  // people_count changes when someone gets a role or is registered.
  const reloadRoles = useCallback(() => {
    if (!(has(me, "manage_people") || has(me, "manage_roles"))) return
    api.roles().then(setRoles, () => {})
  }, [me])

  useEffect(() => {
    if (canSeeTeam(me)) load()
  }, [me, load])

  if (!canSeeTeam(me)) return <Navigate to="/" replace />

  const tabs: Tab[] = [...(canPeople ? (["pessoas"] as Tab[]) : []), "areas", ...(canRoles ? (["cargos", "acessos"] as Tab[]) : [])]
  const asked = params.get("aba") as Tab | null
  const tab = asked && tabs.includes(asked) ? asked : tabs[0]

  return (
    <>
      <PageHeader title="Equipe" />
      <div className="mb-6 flex gap-2" role="tablist">
        {tabs.map((t) => (
          <Button key={t} role="tab" aria-selected={tab === t} size="sm" variant={tab === t ? "default" : "outline"} onClick={() => setParams({ aba: t })}>
            {TAB_NAMES[t]}
          </Button>
        ))}
      </div>
      {loadError ? (
        <LoadError message={loadError} onRetry={load} />
      ) : !loaded ? (
        <div role="status" aria-label="Carregando equipe" className="space-y-3">
          <div className="h-7 w-48 animate-pulse rounded bg-muted" />
          <div className="h-24 animate-pulse rounded-xl bg-muted" />
          <div className="h-20 animate-pulse rounded-xl bg-muted" />
        </div>
      ) : tab === "pessoas" ? (
        <PeopleTab people={people} setPeople={setPeople} roles={roles} areas={areas} me={me!} reloadRoles={reloadRoles} />
      ) : tab === "areas" ? (
        <AreasTab areas={areas} people={people} reload={load} canAreas={canAreas} leads={leads} />
      ) : tab === "cargos" ? (
        <RolesTab roles={roles} setRoles={setRoles} myRoleId={me!.role_id} />
      ) : (
        <MembershipTab membership={membership} setMembership={setMembership} roles={roles} me={me!} reload={load} />
      )}
    </>
  )
}

function PeopleTab({ people, setPeople, roles, areas, me, reloadRoles }: { people: PersonRow[]; setPeople: (f: (l: PersonRow[]) => PersonRow[]) => void; roles: RoleInfo[]; areas: Area[]; me: Person; reloadRoles: () => void }) {
  const { refresh } = useAuth()
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({})
  const [saving, setSaving] = useState<number | null>(null)
  const [form, setForm] = useState({ name: "", email: "", password: "", role_id: 0 })
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  // Nobody gives a role with more than their own, nor changes the role of someone with more.
  const within = (r: RoleInfo) => r.permissions.every((p) => has(me, p))
  const givable = roles.filter(within)
  const roleId = form.role_id || givable.find((r) => r.key === "usuario")?.id || givable[0]?.id || 0

  const changeRole = async (p: PersonRow, id: number) => {
    setRowErrors((e) => ({ ...e, [p.id]: "" }))
    setSaving(p.id)
    try {
      const updated = await api.setPersonRole(p.id, id)
      setPeople((l) => l.map((x) => (x.id === p.id ? { ...x, role_id: updated.role_id ?? id, role_name: updated.role_name ?? roles.find((r) => r.id === id)?.name ?? x.role_name } : x)))
      reloadRoles()
    } catch (err) {
      setRowErrors((e) => ({ ...e, [p.id]: errText(err, "Não foi possível mudar o cargo.") }))
      return
    } finally {
      setSaving(null)
    }
    // The change is saved; a failed refresh only means the screen is a bit behind.
    if (p.id === me.id) await refresh().catch(() => {})
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setBusy(true)
    try {
      const p = await api.createPerson({ ...form, role_id: roleId })
      setPeople((list) => [...list, p].sort((a, b) => a.name.localeCompare(b.name)))
      setForm({ name: "", email: "", password: "", role_id: 0 })
      reloadRoles()
      toast.success(`${p.name} foi cadastrado(a).`)
    } catch (err) {
      setError(errText(err, "Não foi possível cadastrar."))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid gap-10 lg:grid-cols-[1fr_22rem]">
      <ul className="h-fit divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
        {people.map((p) => {
          const names = areas.filter((a) => p.area_ids.includes(a.id)).map((a) => a.name).join(", ")
          const role = roles.find((r) => r.id === p.role_id)
          return (
            <li key={p.id} className="px-5 py-3.5">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
                <div className="flex min-w-0 flex-1 items-center gap-4">
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold">{p.name.slice(0, 1).toUpperCase()}</span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-medium">
                      {p.name} {p.id === me.id && <span className="text-xs font-normal text-muted-foreground">(você)</span>}
                    </p>
                    <p className="truncate text-sm text-muted-foreground">{p.email}</p>
                    <p className="truncate text-sm text-muted-foreground">{names || "Sem área"}</p>
                  </div>
                </div>
                {role && !within(role) ? (
                  <p className="text-sm sm:w-44">
                    <span className="sr-only">Cargo: </span>
                    {p.role_name}
                  </p>
                ) : (
                  <NativeSelect aria-label={`Cargo de ${p.name}`} value={p.role_id} disabled={saving === p.id} onChange={(e) => void changeRole(p, Number(e.target.value))} className="sm:w-44">
                    {givable.map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.name}
                      </option>
                    ))}
                  </NativeSelect>
                )}
              </div>
              {rowErrors[p.id] && (
                <p role="alert" className="mt-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {rowErrors[p.id]}
                </p>
              )}
            </li>
          )
        })}
      </ul>

      <form onSubmit={submit} className="flex h-fit flex-col gap-4 rounded-xl border border-border bg-card p-6">
        <h2 className="font-heading text-xl">Cadastrar pessoa</h2>
        <Field label="Nome" htmlFor="p-name">
          <Input id="p-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        </Field>
        <Field label="E-mail" htmlFor="p-email">
          <Input id="p-email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
        </Field>
        <Field label="Senha inicial" htmlFor="p-pass" hint="Pelo menos 8 caracteres. Passe para a pessoa pessoalmente.">
          <Input id="p-pass" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required autoComplete="new-password" />
        </Field>
        <Field label="Cargo" htmlFor="p-role">
          <NativeSelect id="p-role" value={roleId} onChange={(e) => setForm({ ...form, role_id: Number(e.target.value) })}>
            {givable.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </NativeSelect>
        </Field>
        {error && (
          <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}
        <Button type="submit" size="lg" disabled={busy} className="h-10">
          {busy ? "Salvando…" : "Cadastrar"}
        </Button>
      </form>
    </div>
  )
}

type Editing = { id: number; kind: "rename" | "leaders" | "members" } | null

function AreasTab({ areas, people, reload, canAreas, leads }: { areas: Area[]; people: PersonRow[]; reload: () => void; canAreas: boolean; leads: number[] }) {
  const [editing, setEditing] = useState<Editing>(null)
  const [name, setName] = useState("")
  const [picked, setPicked] = useState<number[]>([])
  const [newName, setNewName] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setError("")
    setBusy(true)
    try {
      await fn()
      after?.()
      setEditing(null)
      reload()
    } catch (err) {
      setError(errText(err, "Não foi possível salvar."))
    } finally {
      setBusy(false)
    }
  }

  const open = (a: Area, kind: "rename" | "leaders" | "members") => {
    setError("")
    setEditing({ id: a.id, kind })
    setName(a.name)
    setPicked((kind === "leaders" ? a.leaders : a.members).map((x) => x.id))
  }
  const toggle = (id: number) => setPicked((l) => (l.includes(id) ? l.filter((x) => x !== id) : [...l, id]))

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
        {areas.map((a) => {
          const canMembers = canAreas || leads.includes(a.id)
          const isEditing = editing?.id === a.id
          return (
            <li key={a.id} className="px-5 py-3.5">
              <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
                <p className="font-medium">{a.name}</p>
                <p className="text-sm text-muted-foreground">{count(a.app_count, "app", "apps")}</p>
                <p className="text-sm text-muted-foreground">Líderes: {a.leaders.map((x) => x.name).join(", ") || "ninguém"}</p>
                <p className="text-sm text-muted-foreground">Membros: {a.members.map((x) => x.name).join(", ") || "ninguém"}</p>
              </div>
              <div className="mt-1 flex flex-wrap gap-4">
                {canAreas && (
                  <button type="button" className={textBtn} aria-label={`Renomear ${a.name}`} onClick={() => open(a, "rename")}>
                    Renomear
                  </button>
                )}
                {canAreas && a.app_count === 0 && (
                  <button
                    type="button"
                    className={textBtn}
                    aria-label={`Apagar ${a.name}`}
                    onClick={() => {
                      if (window.confirm(`Apagar a área ${a.name}?`)) void run(() => api.deleteArea(a.id))
                    }}
                  >
                    Apagar
                  </button>
                )}
                {canAreas && (
                  <button type="button" className={textBtn} aria-label={`Líderes de ${a.name}`} onClick={() => open(a, "leaders")}>
                    Líderes
                  </button>
                )}
                {canMembers && (
                  <button type="button" className={textBtn} aria-label={`Membros de ${a.name}`} onClick={() => open(a, "members")}>
                    Membros
                  </button>
                )}
              </div>
              {isEditing && editing.kind === "rename" && (
                <form
                  className="mt-3 flex items-end gap-2"
                  onSubmit={(e) => {
                    e.preventDefault()
                    void run(() => api.renameArea(a.id, name.trim()))
                  }}
                >
                  <Input aria-label={`Novo nome de ${a.name}`} value={name} onChange={(e) => setName(e.target.value)} required />
                  <Button type="submit" size="sm" disabled={busy}>
                    Salvar
                  </Button>
                  <Button type="button" size="sm" variant="outline" onClick={() => setEditing(null)}>
                    Cancelar
                  </Button>
                </form>
              )}
              {isEditing && editing.kind !== "rename" && (
                <div className="mt-3 flex flex-col gap-2">
                  <p className="text-sm font-medium">{editing.kind === "leaders" ? "Quem lidera esta área" : "Quem faz parte desta área"}</p>
                  <div className="grid gap-1 sm:grid-cols-2">
                    {people.map((p) => (
                      <label key={p.id} className="flex items-center gap-2 text-sm">
                        <input type="checkbox" checked={picked.includes(p.id)} onChange={() => toggle(p.id)} />
                        {p.name}
                      </label>
                    ))}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      size="sm"
                      disabled={busy}
                      onClick={() => void run(() => (editing.kind === "leaders" ? api.setAreaLeaders(a.id, picked) : api.setAreaMembers(a.id, picked)))}
                    >
                      Salvar
                    </Button>
                    <Button type="button" size="sm" variant="outline" onClick={() => setEditing(null)}>
                      Cancelar
                    </Button>
                  </div>
                </div>
              )}
            </li>
          )
        })}
      </ul>
      {error && (
        <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}
      {canAreas && (
        <form
          className="flex items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            void run(() => api.createArea(newName.trim()), () => setNewName(""))
          }}
        >
          <Field label="Nova área" htmlFor="new-area">
            <Input id="new-area" value={newName} onChange={(e) => setNewName(e.target.value)} required />
          </Field>
          <Button type="submit" disabled={busy}>
            Criar área
          </Button>
        </form>
      )}
    </div>
  )
}

function RoleForm({ initial, submitLabel, onSubmit, onCancel }: { initial: { name: string; permissions: string[] }; submitLabel: string; onSubmit: (b: { name: string; permissions: string[] }) => Promise<void>; onCancel: () => void }) {
  const [name, setName] = useState(initial.name)
  const [perms, setPerms] = useState(initial.permissions)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const toggle = (k: string) => setPerms((l) => (l.includes(k) ? l.filter((x) => x !== k) : [...l, k]))

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError("")
    try {
      await onSubmit({ name: name.trim(), permissions: perms })
    } catch (err) {
      setError(errText(err, "Não foi possível salvar."))
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="mt-3 flex flex-col gap-3">
      <Field label="Nome do cargo" htmlFor="role-name">
        <Input id="role-name" value={name} onChange={(e) => setName(e.target.value)} required />
      </Field>
      <div className="grid gap-2 sm:grid-cols-2">
        {PERMISSIONS.map((p) => (
          <div key={p.key} className="flex items-start gap-2 text-sm">
            <input id={`perm-${p.key}`} type="checkbox" className="mt-1" checked={perms.includes(p.key)} onChange={() => toggle(p.key)} aria-describedby={`hint-${p.key}`} />
            <div>
              <label htmlFor={`perm-${p.key}`}>{p.label}</label>
              <p id={`hint-${p.key}`} className="text-xs text-muted-foreground">
                {p.hint}
              </p>
            </div>
          </div>
        ))}
      </div>
      {error && (
        <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={busy}>
          {submitLabel}
        </Button>
        <Button type="button" size="sm" variant="outline" onClick={onCancel}>
          Cancelar
        </Button>
      </div>
    </form>
  )
}

function RolesTab({ roles, setRoles, myRoleId }: { roles: RoleInfo[]; setRoles: (f: (l: RoleInfo[]) => RoleInfo[]) => void; myRoleId: number }) {
  const { refresh } = useAuth()
  const [editingId, setEditingId] = useState<number | "new" | null>(null)
  const [error, setError] = useState("")

  const remove = async (r: RoleInfo) => {
    if (!window.confirm(`Apagar o cargo ${r.name}?`)) return
    setError("")
    try {
      await api.deleteRole(r.id)
      setRoles((l) => l.filter((x) => x.id !== r.id))
    } catch (err) {
      setError(errText(err, "Não foi possível apagar o cargo."))
    }
  }

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
        {roles.map((r) => (
          <li key={r.id} className="px-5 py-3.5">
            <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
              <p className="font-medium">{r.name}</p>
              <p className="text-sm text-muted-foreground">{count(r.people_count, "pessoa", "pessoas")}</p>
            </div>
            <p className="text-sm text-muted-foreground">{r.permissions.length ? r.permissions.map(permLabel).join(" · ") : "Nenhuma permissão"}</p>
            <div className="mt-1 flex gap-4">
              <button type="button" className={textBtn} aria-label={`Editar ${r.name}`} onClick={() => setEditingId(r.id)}>
                Editar
              </button>
              {r.key === "" && r.people_count === 0 && (
                <button type="button" className={textBtn} aria-label={`Apagar ${r.name}`} onClick={() => void remove(r)}>
                  Apagar
                </button>
              )}
            </div>
            {editingId === r.id && (
              <RoleForm
                initial={r}
                submitLabel="Salvar"
                onCancel={() => setEditingId(null)}
                onSubmit={async (b) => {
                  const saved = await api.updateRole(r.id, b)
                  setRoles((l) => l.map((x) => (x.id === r.id ? saved : x)))
                  setEditingId(null)
                  // My own boxes changed: the tabs and menus follow them.
                  if (r.id === myRoleId) await refresh().catch(() => {})
                }}
              />
            )}
          </li>
        ))}
      </ul>
      {error && (
        <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}
      {editingId === "new" ? (
        <div className="rounded-xl border border-border bg-card p-5">
          <h2 className="font-heading text-xl">Novo cargo</h2>
          <RoleForm
            initial={{ name: "", permissions: [] }}
            submitLabel="Criar cargo"
            onCancel={() => setEditingId(null)}
            onSubmit={async (b) => {
              const created = await api.createRole(b)
              setRoles((l) => [...l, created])
              setEditingId(null)
            }}
          />
        </div>
      ) : (
        <div>
          <Button variant="outline" onClick={() => setEditingId("new")}>
            Novo cargo
          </Button>
        </div>
      )}
    </div>
  )
}

function invitationHref(url: string) {
  const parsed = new URL(url, window.location.origin)
  if (parsed.origin !== window.location.origin) throw new Error("O endereço do convite não pertence a este Ziv.")
  if (parsed.searchParams.has("token")) throw new Error("O convite veio com um formato inseguro. Tente gerar outro link.")
  const base = workspaceBasePath()
  const isWorkspaceRoute = parsed.pathname === base || parsed.pathname.startsWith(`${base}/`)
  const path = base && !isWorkspaceRoute ? `${base}${parsed.pathname.startsWith("/") ? "" : "/"}${parsed.pathname}` : parsed.pathname
  if (!new URLSearchParams(parsed.hash.replace(/^#/, "")).has("token")) throw new Error("O servidor não retornou um link de convite válido.")
  return `${window.location.origin}${path}${parsed.search}${parsed.hash}`
}

function dateLabel(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("pt-BR", { dateStyle: "medium", timeStyle: "short" }).format(date)
}

function MembershipTab({ membership, setMembership, roles, me, reload }: { membership: Membership; setMembership: (membership: Membership) => void; roles: RoleInfo[]; me: Person; reload: () => void }) {
  const [email, setEmail] = useState("")
  const [roleId, setRoleId] = useState(0)
  const [requestRoles, setRequestRoles] = useState<Record<number, number>>({})
  const [inviteUrl, setInviteUrl] = useState("")
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState("")
  const canGrant = (role: RoleInfo) => role.permissions.every((permission) => has(me, permission))
  const givable = roles.filter(canGrant)
  const defaultRoleId = givable.find((role) => role.key === "usuario")?.id ?? 0
  const selectedRoleId = roleId || defaultRoleId

  const showInvitation = (url: string) => {
    setInviteUrl(invitationHref(url))
    setCopied(false)
  }

  const createInvite = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    setBusy("create")
    try {
      const result = await api.createMembershipInvitation({ email: email.trim(), role_id: selectedRoleId })
      showInvitation(result.url)
      setEmail("")
      toast.success("Convite criado. Compartilhe o link com a pessoa.")
      reload()
    } catch (reason) {
      setError(errText(reason, "Não foi possível criar o convite."))
    } finally {
      setBusy("")
    }
  }

  const approve = async (id: number) => {
    setError("")
    setBusy(`approve-${id}`)
    try {
      const result = await api.approveMembershipRequest(id, requestRoles[id] || defaultRoleId)
      showInvitation(result.url)
      setMembership({ ...membership, requests: membership.requests.filter((request) => request.id !== id) })
      toast.success("A solicitação foi aprovada. O convite ainda precisa ser aceito.")
      reload()
    } catch (reason) {
      setError(errText(reason, "Não foi possível aprovar a solicitação."))
    } finally {
      setBusy("")
    }
  }

  const reject = async (id: number, name: string) => {
    if (!window.confirm(`Recusar a solicitação de ${name}?`)) return
    setError("")
    setBusy(`reject-${id}`)
    try {
      await api.rejectMembershipRequest(id)
      setMembership({ ...membership, requests: membership.requests.filter((request) => request.id !== id) })
      toast.success("Solicitação recusada.")
    } catch (reason) {
      setError(errText(reason, "Não foi possível recusar a solicitação."))
    } finally {
      setBusy("")
    }
  }

  const revoke = async (id: number, address: string) => {
    if (!window.confirm(`Revogar o convite enviado para ${address}?`)) return
    setError("")
    setBusy(`revoke-${id}`)
    try {
      await api.revokeMembershipInvitation(id)
      setMembership({ ...membership, invitations: membership.invitations.filter((invitation) => invitation.id !== id) })
      setInviteUrl("")
      setCopied(false)
      toast.success("Convite revogado.")
    } catch (reason) {
      setError(errText(reason, "Não foi possível revogar o convite."))
    } finally {
      setBusy("")
    }
  }

  const copyUrl = async () => {
    if (!inviteUrl) return
    try {
      await navigator.clipboard.writeText(inviteUrl)
      setCopied(true)
    } catch {
      setCopied(false)
      setError("Não consegui copiar automaticamente. Selecione o endereço e copie manualmente.")
    }
  }

  return (
    <div className="flex max-w-4xl flex-col gap-6">
      <section className="grid gap-5 rounded-xl border border-logo-square/15 bg-logo-square/[0.035] p-5 sm:grid-cols-[1fr_auto] sm:items-center">
        <div>
          <p className="flex items-center gap-2 font-heading text-lg"><MailPlus className="size-4 text-logo-square" /> Convide alguém</p>
          <p className="mt-1 max-w-xl text-sm leading-5 text-muted-foreground">O link vale por 7 dias e só pode ser usado uma vez. O Ziv não envia e-mail: copie o endereço e envie ao e-mail da pessoa por um canal confiável. Quem tiver o link poderá ativar o acesso; não o publique em grupos.</p>
        </div>
        <span className="inline-flex items-center gap-2 text-xs font-medium text-muted-foreground"><Link2 className="size-3.5" /> Acesso restrito à empresa</span>
      </section>

      <form onSubmit={createInvite} className="grid gap-4 rounded-xl border border-border bg-card p-5 sm:grid-cols-[1fr_12rem_auto] sm:items-end">
        <Field label="E-mail da pessoa" htmlFor="invite-email" hint="O convite fica vinculado a este endereço.">
          <Input id="invite-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" required />
        </Field>
        <Field label="Cargo inicial" htmlFor="invite-role">
          <NativeSelect id="invite-role" value={selectedRoleId} onChange={(event) => setRoleId(Number(event.target.value))}>
            {givable.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}
          </NativeSelect>
        </Field>
        <Button type="submit" className="gap-2" disabled={!selectedRoleId || Boolean(busy)}>{busy === "create" ? <RefreshCw className="size-4 animate-spin" /> : <MailPlus className="size-4" />}{busy === "create" ? "Criando…" : "Criar convite"}</Button>
      </form>
      {!defaultRoleId && <p role="alert" className="rounded-md bg-warn-soft px-3 py-2 text-sm text-warn">O cargo padrão “Usuário” não está disponível para esta conta. Crie ou libere esse cargo antes de enviar convites.</p>}

      {inviteUrl && (
        <section className="rounded-xl border border-ok/25 bg-ok-soft p-4 sm:p-5">
          <p className="flex items-center gap-2 font-medium text-ok"><Check className="size-4" /> Link de convite pronto</p>
          <p className="mt-1 text-sm text-foreground/80">Envie este endereço manualmente. Ele contém a chave de uso único; não o publique em canais abertos.</p>
          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            <Input aria-label="Endereço do convite" value={inviteUrl} readOnly onFocus={(event) => event.currentTarget.select()} className="font-mono text-xs" />
            <Button type="button" variant="outline" className="gap-2" onClick={() => void copyUrl()}><Copy className="size-4" />{copied ? "Copiado" : "Copiar link"}</Button>
          </div>
        </section>
      )}

      {error && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}

      <section>
        <div className="mb-3 flex items-end justify-between gap-4">
          <div><h2 className="font-heading text-xl">Solicitações pendentes</h2><p className="mt-1 text-sm text-muted-foreground">Aprovar gera um convite; a pessoa só entra depois de aceitá-lo e criar a senha.</p></div>
          <span className="font-mono text-sm text-muted-foreground">{membership.requests.length.toString().padStart(2, "0")}</span>
        </div>
        {membership.requests.length ? (
          <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
            {membership.requests.map((request) => {
              const selected = requestRoles[request.id] || defaultRoleId
              return <li key={request.id} className="grid gap-4 p-4 sm:grid-cols-[1fr_12rem_auto] sm:items-center">
                <div className="min-w-0">
                  <p className="truncate font-medium">{request.name}</p>
                  <p className="truncate text-sm text-muted-foreground">{request.email}</p>
                  <p className="mt-1 text-xs text-muted-foreground">Solicitou em {dateLabel(request.created_at)}</p>
                </div>
                <NativeSelect aria-label={`Cargo do convite para ${request.name}`} value={selected} onChange={(event) => setRequestRoles({ ...requestRoles, [request.id]: Number(event.target.value) })}>
                  {givable.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}
                </NativeSelect>
                <div className="flex gap-2">
                  <Button type="button" size="sm" disabled={!selected || Boolean(busy)} onClick={() => void approve(request.id)} className="gap-1.5"><UserCheck className="size-4" /> Aprovar</Button>
                  <Button type="button" size="sm" variant="outline" disabled={Boolean(busy)} onClick={() => void reject(request.id, request.name)} aria-label={`Recusar solicitação de ${request.name}`}><UserX className="size-4" /></Button>
                </div>
              </li>
            })}
          </ul>
        ) : (
          <div className="rounded-xl border border-dashed border-input bg-card/50 px-5 py-7">
            <p className="font-medium">Nenhuma solicitação aguardando análise</p>
            <p className="mt-1 text-sm text-muted-foreground">Quando alguém pedir acesso por esta página da empresa, a solicitação aparecerá aqui.</p>
          </div>
        )}
      </section>

      <section>
        <div className="mb-3 flex items-end justify-between gap-4">
          <div><h2 className="font-heading text-xl">Convites ativos</h2><p className="mt-1 text-sm text-muted-foreground">Convites não aceitos podem ser revogados a qualquer momento.</p></div>
          <span className="font-mono text-sm text-muted-foreground">{membership.invitations.length.toString().padStart(2, "0")}</span>
        </div>
        {membership.invitations.length ? (
          <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
            {membership.invitations.map((invitation) => <li key={invitation.id} className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center">
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{invitation.email}</p>
                <p className="text-sm text-muted-foreground">{invitation.role_name} · vence em {dateLabel(invitation.expires_at)}</p>
              </div>
              <Button type="button" aria-label={`Revogar convite de ${invitation.email}`} variant="outline" size="sm" disabled={Boolean(busy)} onClick={() => void revoke(invitation.id, invitation.email)}>Revogar</Button>
            </li>)}
          </ul>
        ) : (
          <div className="rounded-xl border border-dashed border-input bg-card/50 px-5 py-7">
            <p className="font-medium">Sem convites ativos</p>
            <p className="mt-1 text-sm text-muted-foreground">Convites criados aparecerão aqui até serem aceitos, expirarem ou serem revogados.</p>
          </div>
        )}
      </section>
    </div>
  )
}
