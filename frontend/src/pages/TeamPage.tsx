import { useCallback, useEffect, useState, type FormEvent } from "react"
import { Navigate, useSearchParams } from "react-router-dom"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, LoadError, NativeSelect, PageHeader } from "@/components/bits"
import { api, type Area, type PersonRow, type RoleInfo } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { PERMISSIONS, canSeeTeam, has, permLabel } from "@/lib/permissions"

type Tab = "pessoas" | "areas" | "cargos"
const TAB_NAMES: Record<Tab, string> = { pessoas: "Pessoas", areas: "Áreas", cargos: "Cargos" }
const textBtn = "text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
const errText = (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback)

export function TeamPage() {
  const { person: me } = useAuth()
  const [params, setParams] = useSearchParams()
  const [people, setPeople] = useState<PersonRow[]>([])
  const [roles, setRoles] = useState<RoleInfo[]>([])
  const [areas, setAreas] = useState<Area[]>([])
  const [loadError, setLoadError] = useState("")
  const [loaded, setLoaded] = useState(false)

  const canPeople = has(me, "manage_people")
  const canAreas = has(me, "manage_areas")
  const canRoles = has(me, "manage_roles")
  const leads = me?.leads ?? []

  const load = useCallback(() => {
    setLoadError("")
    const wantPeople = has(me, "manage_people") || has(me, "manage_areas") || (me?.leads.length ?? 0) > 0
    Promise.all([
      wantPeople ? api.people() : Promise.resolve([] as PersonRow[]),
      has(me, "manage_people") || has(me, "manage_roles") ? api.roles() : Promise.resolve([] as RoleInfo[]),
      api.areas(),
    ]).then(
      ([p, r, a]) => {
        setPeople(p)
        setRoles(r)
        setAreas(a)
        setLoaded(true)
      },
      (e: unknown) => setLoadError(errText(e, "Não foi possível carregar a equipe.")),
    )
  }, [me])

  useEffect(() => {
    if (canSeeTeam(me)) load()
  }, [me, load])

  if (!canSeeTeam(me)) return <Navigate to="/" replace />

  const tabs: Tab[] = [...(canPeople ? (["pessoas"] as Tab[]) : []), "areas", ...(canRoles ? (["cargos"] as Tab[]) : [])]
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
      ) : !loaded ? null : tab === "pessoas" ? (
        <PeopleTab people={people} setPeople={setPeople} roles={roles} areas={areas} meId={me!.id} />
      ) : tab === "areas" ? (
        <AreasTab areas={areas} people={people} reload={load} canAreas={canAreas} leads={leads} />
      ) : (
        <RolesTab roles={roles} setRoles={setRoles} />
      )}
    </>
  )
}

function PeopleTab({ people, setPeople, roles, areas, meId }: { people: PersonRow[]; setPeople: (f: (l: PersonRow[]) => PersonRow[]) => void; roles: RoleInfo[]; areas: Area[]; meId: number }) {
  const { refresh } = useAuth()
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({})
  const [form, setForm] = useState({ name: "", email: "", password: "", role_id: 0 })
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const roleId = form.role_id || roles.find((r) => r.key === "usuario")?.id || roles[0]?.id || 0

  const changeRole = async (p: PersonRow, id: number) => {
    setRowErrors((e) => ({ ...e, [p.id]: "" }))
    try {
      const updated = await api.setPersonRole(p.id, id)
      setPeople((l) => l.map((x) => (x.id === p.id ? { ...x, role_id: updated.role_id ?? id, role_name: updated.role_name ?? roles.find((r) => r.id === id)?.name ?? x.role_name } : x)))
      if (p.id === meId) await refresh()
    } catch (err) {
      setRowErrors((e) => ({ ...e, [p.id]: errText(err, "Não foi possível mudar o cargo.") }))
    }
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setBusy(true)
    try {
      const p = await api.createPerson({ ...form, role_id: roleId })
      setPeople((list) => [...list, p].sort((a, b) => a.name.localeCompare(b.name)))
      setForm({ name: "", email: "", password: "", role_id: 0 })
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
          return (
            <li key={p.id} className="px-5 py-3.5">
              <div className="flex items-center gap-4">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold">{p.name.slice(0, 1).toUpperCase()}</span>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">
                    {p.name} {p.id === meId && <span className="text-xs font-normal text-muted-foreground">(você)</span>}
                  </p>
                  <p className="truncate text-sm text-muted-foreground">{p.email}</p>
                  <p className="truncate text-sm text-muted-foreground">{names || "Sem área"}</p>
                </div>
                <NativeSelect aria-label={`Cargo de ${p.name}`} value={p.role_id} onChange={(e) => void changeRole(p, Number(e.target.value))} className="w-44">
                  {roles.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
                </NativeSelect>
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
            {roles.map((r) => (
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
                <p className="text-sm text-muted-foreground">{a.app_count} apps</p>
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

function RolesTab({ roles, setRoles }: { roles: RoleInfo[]; setRoles: (f: (l: RoleInfo[]) => RoleInfo[]) => void }) {
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
              <p className="text-sm text-muted-foreground">{r.people_count} pessoas</p>
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
