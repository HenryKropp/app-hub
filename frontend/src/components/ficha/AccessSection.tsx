import { useEffect, useId, useState } from "react"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { NativeSelect } from "@/components/bits"
import { api, type AppAccess, type AppDetail, type Area, type Grant, type Person, type PersonRow } from "@/lib/api"
import { GRANTS, permLabel } from "@/lib/permissions"
import { EditLink, SaveBar, Section } from "./Part"

function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (on: boolean) => void }) {
  const id = useId()
  return (
    <div className="flex items-center gap-2 text-sm">
      <Checkbox aria-labelledby={id} checked={checked} onCheckedChange={(c) => onChange(c === true)} />
      <span id={id} className="cursor-default" onClick={() => onChange(!checked)}>{label}</span>
    </div>
  )
}

const names = (xs: { name: string }[]) => {
  const n = xs.map((x) => x.name)
  return n.length <= 1 ? n.join("") : `${n.slice(0, -1).join(", ")} e ${n[n.length - 1]}`
}

type Viewers = { restricted: boolean; area_ids: number[]; person_ids: number[] }
type GrantForm = { person_id: number; permissions: string[]; whole_area: boolean }
const BLANK: GrantForm = { person_id: 0, permissions: [], whole_area: false }

// AccessSection says who sees the app and who has extra accesses. Those who
// take care of access (can.access) change who sees it; only leaders (can.grants)
// give and take extra accesses.
export function AccessSection({ app, me }: { app: AppDetail; me: Person }) {
  const { can } = app
  const [access, setAccess] = useState<AppAccess | null>(null)
  const [error, setError] = useState("")
  const [areas, setAreas] = useState<Area[]>([])
  const [people, setPeople] = useState<PersonRow[]>([])
  const [viewers, setViewers] = useState<Viewers | null>(null)
  const [form, setForm] = useState<GrantForm | null>(null)
  const [formError, setFormError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let gone = false
    api.appAccess(app.id).then(
      (a) => {
        if (gone) return
        setAccess(a)
        if (can.grants && a.grants.length === 0 && a.area_grants.length === 0) setForm(BLANK)
      },
      (e: Error) => !gone && setError(e.message),
    )
    if (can.access || can.grants) {
      api.areas().then((x) => !gone && setAreas(x), () => {})
      api.people().then((x) => !gone && setPeople(x), () => {})
    }
    return () => {
      gone = true
    }
  }, [app.id, can.access, can.grants])

  if (error) return <Section title="Acesso"><p className="text-sm text-muted-foreground">{error}</p></Section>
  if (!access) return <Section title="Acesso"><p className="text-sm text-muted-foreground">Carregando…</p></Section>

  const current: Viewers = { restricted: access.restricted, area_ids: access.areas.map((a) => a.id), person_ids: access.people.map((p) => p.id) }
  const viewersDirty = !!viewers && JSON.stringify(viewers) !== JSON.stringify(current)
  const saveViewers = async () => {
    const v = viewers!
    const saved = await api.setAppAccess(app.id, v.restricted ? v : { restricted: false, area_ids: [], person_ids: [] })
    setAccess(saved)
    setViewers(null)
  }
  const toggle = (xs: number[], id: number, on: boolean) => (on ? [...xs, id] : xs.filter((x) => x !== id))

  const giveGrant = async (g: GrantForm) => {
    setBusy(true)
    setFormError("")
    try {
      setAccess(await api.setGrants(app.id, g))
      return true
    } catch (e) {
      setFormError(e instanceof Error ? e.message : "Não consegui salvar.")
      return false
    } finally {
      setBusy(false)
    }
  }
  const submitForm = async () => {
    if (!form) return
    if (!form.person_id) return setFormError("Escolha a pessoa.")
    if (form.permissions.length === 0) return setFormError("Marque ao menos um acesso.")
    if (await giveGrant(form)) setForm(null)
  }
  const grantRow = (g: Grant, whole: boolean) => {
    const who = whole ? `${g.name} (área)` : g.name
    return (
    <li key={`${whole ? "a" : "p"}-${g.person_id}`} className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
      <span>
        {g.name}: {g.permissions.map(permLabel).join(", ")}
        {whole && <span className="text-muted-foreground"> · todos os apps da área</span>}
      </span>
      {can.grants && (
        <span className="flex gap-3 text-sm">
          <button type="button" className="text-muted-foreground hover:text-foreground" aria-label={`Mudar ${who}`}
            onClick={() => { setFormError(""); setForm({ person_id: g.person_id, permissions: g.permissions, whole_area: whole }) }}>
            Mudar
          </button>
          <button type="button" className="text-muted-foreground hover:text-destructive" aria-label={`Tirar ${who}`} disabled={busy}
            onClick={() => void giveGrant({ person_id: g.person_id, permissions: [], whole_area: whole })}>
            Tirar
          </button>
        </span>
      )}
    </li>
    )
  }

  const chosen = viewers ? people.filter((p) => viewers.person_ids.includes(p.id)) : []
  const noExtras = access.grants.length === 0 && access.area_grants.length === 0

  return (
    <Section title="Acesso">
      <div className="flex flex-col gap-5 text-sm">
        {viewers ? (
          <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-xs">
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-1 font-medium">Quem vê</legend>
              <label className="flex items-center gap-2">
                <input type="radio" name={`viewers-${app.id}`} className="accent-foreground" checked={!viewers.restricted} onChange={() => setViewers({ ...viewers, restricted: false })} />
                Todos
              </label>
              <label className="flex items-center gap-2">
                <input type="radio" name={`viewers-${app.id}`} className="accent-foreground" checked={viewers.restricted} onChange={() => setViewers({ ...viewers, restricted: true })} />
                Só algumas áreas e pessoas
              </label>
            </fieldset>
            {viewers.restricted && (
              <div className="flex flex-col gap-3 pl-6">
                <div className="flex flex-col gap-1.5">
                  {areas.map((a) => (
                    <Check key={a.id} label={a.name} checked={viewers.area_ids.includes(a.id)} onChange={(on) => setViewers({ ...viewers, area_ids: toggle(viewers.area_ids, a.id, on) })} />
                  ))}
                </div>
                {chosen.length > 0 && (
                  <ul className="flex flex-col gap-1">
                    {chosen.map((p) => (
                      <li key={p.id} className="flex items-center gap-2">
                        {p.name}
                        <Button variant="ghost" size="icon" className="size-7" aria-label={`Tirar ${p.name} de quem vê`} onClick={() => setViewers({ ...viewers, person_ids: viewers.person_ids.filter((x) => x !== p.id) })}>
                          <X />
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
                <NativeSelect aria-label="Adicionar pessoa" className="max-w-xs" value={0}
                  onChange={(e) => { const id = Number(e.target.value); if (id) setViewers({ ...viewers, person_ids: [...viewers.person_ids, id] }) }}>
                  <option value={0}>Adicionar pessoa…</option>
                  {people.filter((p) => !viewers.person_ids.includes(p.id)).map((p) => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </NativeSelect>
              </div>
            )}
            <SaveBar dirty={viewersDirty} onSave={saveViewers} onCancel={() => setViewers(null)} />
          </div>
        ) : (
          <div>
            <p className="flex flex-wrap items-baseline gap-x-3">
              <span>Quem vê: {access.restricted ? `só ${names([...access.areas, ...access.people]) || "ninguém além de quem cuida"}` : "Todos"}</span>
              {can.access && <EditLink part="Quem vê" onClick={() => setViewers(current)} />}
            </p>
            {access.restricted && <p className="mt-1 text-muted-foreground">Líderes, responsável e reserva sempre veem.</p>}
          </div>
        )}

        <div>
          <p className="mb-1.5">Acessos extras:</p>
          {noExtras ? (
            <p className="text-muted-foreground">Ninguém tem acesso extra.</p>
          ) : (
            <ul className="flex flex-col gap-1.5">
              {access.grants.map((g) => grantRow(g, false))}
              {access.area_grants.map((g) => grantRow(g, true))}
            </ul>
          )}
          {can.grants && !form && (
            <button type="button" className="mt-2 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline" onClick={() => { setFormError(""); setForm(BLANK) }}>
              Dar acesso extra
            </button>
          )}
          {!form && formError && <p role="alert" className="mt-2 text-destructive">{formError}</p>}
        </div>

        {can.grants && form && (
          <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-xs">
            <h3 className="font-medium">Dar acesso extra</h3>
            <div className="flex flex-col gap-1.5">
              <label htmlFor={`grant-person-${app.id}`} className="font-medium">Pessoa</label>
              <NativeSelect id={`grant-person-${app.id}`} className="max-w-xs" value={form.person_id} onChange={(e) => setForm({ ...form, person_id: Number(e.target.value) })}>
                <option value={0}>Escolha…</option>
                {people.filter((p) => p.id !== me.id || p.id === form.person_id).map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </NativeSelect>
            </div>
            <div className="flex flex-col gap-1.5">
              {GRANTS.map((g) => (
                <Check key={g.key} label={g.label} checked={form.permissions.includes(g.key)}
                  onChange={(on) => setForm({ ...form, permissions: on ? [...form.permissions, g.key] : form.permissions.filter((k) => k !== g.key) })} />
              ))}
            </div>
            <Check label={`Em todos os apps da área ${app.manifest.area}`} checked={form.whole_area} onChange={(on) => setForm({ ...form, whole_area: on })} />
            {formError && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-destructive">{formError}</p>}
            <div className="flex gap-2">
              <Button size="sm" onClick={submitForm} disabled={busy}>{busy ? "Salvando…" : "Dar acesso"}</Button>
              <Button size="sm" variant="ghost" onClick={() => { setFormError(""); setForm(null) }} disabled={busy}>Cancelar</Button>
            </div>
          </div>
        )}
      </div>
    </Section>
  )
}
