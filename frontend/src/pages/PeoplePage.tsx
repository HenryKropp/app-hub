import { useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, NativeSelect, PageHeader } from "@/components/bits"
import { api, type PersonRow, type RoleInfo } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { has } from "@/lib/permissions"

export function PeoplePage() {
  const { person: me } = useAuth()
  const [people, setPeople] = useState<PersonRow[]>([])
  const [roles, setRoles] = useState<RoleInfo[]>([])
  const [form, setForm] = useState({ name: "", email: "", password: "", role_id: 0 })
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api.people().then(setPeople, (e: Error) => setError(e.message))
    api.roles().then(setRoles, () => {})
  }, [])

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
      setError(err instanceof Error ? err.message : "Não foi possível cadastrar.")
    } finally {
      setBusy(false)
    }
  }

  const canManage = has(me, "manage_people")
  const roleId = form.role_id || roles.find((r) => r.key === "usuario")?.id || roles[0]?.id || 0

  return (
    <>
      <PageHeader title="Quem usa e mantém os apps" />
      <div className="grid gap-10 lg:grid-cols-[1fr_22rem]">
        <ul className="h-fit divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
          {people.map((p) => (
            <li key={p.id} className="flex items-center gap-4 px-5 py-3.5">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold">
                {p.name.slice(0, 1).toUpperCase()}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">
                  {p.name} {p.id === me?.id && <span className="text-xs font-normal text-muted-foreground">(você)</span>}
                </p>
                <p className="truncate text-sm text-muted-foreground">{p.email}</p>
              </div>
              <span className="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-muted-foreground">{p.role_name}</span>
            </li>
          ))}
        </ul>

        {canManage ? (
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
            <Field label="Perfil" htmlFor="p-role" hint={roles.find((r) => r.id === roleId)?.permissions.length === 0 ? "Encontra e usa apps" : undefined}>
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
        ) : (
          <p className="text-sm text-muted-foreground">Só quem cuida das pessoas cadastra pessoas.</p>
        )}
      </div>
    </>
  )
}
