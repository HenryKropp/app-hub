import { useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, NativeSelect, PageHeader } from "@/components/bits"
import { api, type Person, type Role } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { ROLES, roleLabel } from "@/lib/kinds"

export function PeoplePage() {
  const { person: me } = useAuth()
  const [people, setPeople] = useState<Person[]>([])
  const [form, setForm] = useState({ name: "", email: "", password: "", role: "usuario" as Role })
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api.people().then(setPeople, (e: Error) => setError(e.message))
  }, [])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setBusy(true)
    try {
      const p = await api.createPerson(form)
      setPeople((list) => [...list, p].sort((a, b) => a.name.localeCompare(b.name)))
      setForm({ name: "", email: "", password: "", role: "usuario" })
      toast.success(`${p.name} foi cadastrado(a).`)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não foi possível cadastrar.")
    } finally {
      setBusy(false)
    }
  }

  const isAdmin = me?.role === "admin"

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
              <span className="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-muted-foreground">{roleLabel(p.role)}</span>
            </li>
          ))}
        </ul>

        {isAdmin ? (
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
            <Field label="Perfil" htmlFor="p-role" hint={ROLES.find((r) => r.value === form.role)?.hint}>
              <NativeSelect id="p-role" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value as Role })}>
                {ROLES.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
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
          <p className="text-sm text-muted-foreground">Só administradores cadastram pessoas.</p>
        )}
      </div>
    </>
  )
}
