import { useEffect, useState, type FormEvent } from "react"
import { ArrowRight, CircleAlert, ShieldCheck } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { api } from "@/lib/api"

type Preview = { email: string; company_name: string }

function invitationToken() {
  const hash = window.location.hash.startsWith("#") ? window.location.hash.slice(1) : window.location.hash
  return new URLSearchParams(hash).get("token") || ""
}

function safeRedirect(url: string) {
  const target = new URL(url, window.location.origin)
  if (target.origin !== window.location.origin) throw new Error("O destino do convite não pertence a este Ziv.")
  return `${target.pathname}${target.search}${target.hash}`
}

export function InvitationPage() {
  const [token] = useState(invitationToken)
  const [preview, setPreview] = useState<Preview | null>(null)
  const [loading, setLoading] = useState(() => Boolean(token))
  const [loadError, setLoadError] = useState(() => token ? "" : "Este link não contém um convite. Abra o endereço completo recebido da equipe.")
  const [name, setName] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let current = true
    if (!token) return () => { current = false }
    api.previewInvitation(token).then((result) => {
      if (current) setPreview(result)
    }, (reason: unknown) => {
      if (current) setLoadError(reason instanceof Error ? reason.message : "Não foi possível validar o convite.")
    }).finally(() => {
      if (current) setLoading(false)
    })
    return () => { current = false }
  }, [token, attempt])

  const accept = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    const byteLength = new TextEncoder().encode(password).length
    if (byteLength < 8 || byteLength > 72) {
      setError("A senha deve ter entre 8 e 72 bytes.")
      return
    }
    setBusy(true)
    try {
      const result = await api.acceptInvitation({ token, name: name.trim(), password })
      window.location.assign(safeRedirect(result.url))
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Não foi possível aceitar o convite.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="entry-shell relative flex min-h-svh items-center justify-center overflow-hidden px-5 py-10">
      <div className="entry-grid pointer-events-none absolute inset-0 opacity-70" aria-hidden="true" />
      <section className="entry-rise relative w-full max-w-md rounded-2xl border border-border bg-card/95 p-6 shadow-pop backdrop-blur-sm sm:p-8">
        <a href="/boas-vindas" aria-label="Ziv, início" className="inline-flex rounded-sm text-foreground"><Wordmark className="h-9 w-auto" /></a>
        <div className="mt-7 flex size-11 items-center justify-center rounded-xl bg-logo-square text-white"><ShieldCheck className="size-5" /></div>
        {loading ? (
          <div role="status" aria-label="Validando convite" className="mt-6 space-y-3">
            <div className="h-7 w-2/3 animate-pulse rounded bg-muted" />
            <div className="h-4 w-full animate-pulse rounded bg-muted" />
            <div className="h-10 w-full animate-pulse rounded bg-muted" />
            <div className="h-10 w-full animate-pulse rounded bg-muted" />
          </div>
        ) : loadError ? (
          <>
            <h1 className="mt-5 font-heading text-2xl">Não foi possível abrir o convite</h1>
            <p role="alert" className="mt-2 flex gap-2 text-sm text-destructive"><CircleAlert className="mt-0.5 size-4 shrink-0" />{loadError}</p>
            {token && <Button variant="outline" className="mt-5 w-full" onClick={() => { setPreview(null); setLoading(true); setLoadError(""); setAttempt((value) => value + 1) }}>Tentar novamente</Button>}
            <p className="mt-5 text-sm text-muted-foreground">Peça à administração da empresa para confirmar se o convite ainda está ativo.</p>
          </>
        ) : preview ? (
          <>
            <p className="mt-5 text-xs font-semibold uppercase tracking-[0.14em] text-logo-square">Convite da equipe</p>
            <h1 className="mt-2 font-heading text-2xl leading-tight">Entre no espaço {preview.company_name}</h1>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">O convite foi criado para <span className="font-medium text-foreground">{preview.email}</span>. Defina seu nome e uma senha para ativar o acesso.</p>
            <form onSubmit={accept} className="mt-6 flex flex-col gap-4">
              <Field label="Seu nome" htmlFor="invite-name">
                <Input id="invite-name" value={name} onChange={(event) => setName(event.target.value)} autoComplete="name" required />
              </Field>
              <Field label="Crie uma senha" htmlFor="invite-password" hint="Entre 8 e 72 bytes.">
                <Input id="invite-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" minLength={8} required />
              </Field>
              {error && <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
              <Button type="submit" size="lg" className="mt-1 w-full gap-2" disabled={busy}>{busy ? "Ativando acesso…" : "Aceitar convite"} {!busy && <ArrowRight className="size-4" />}</Button>
            </form>
            <p className="mt-5 text-center text-xs leading-5 text-muted-foreground">Este convite é pessoal e pode ser usado uma única vez.</p>
          </>
        ) : null}
      </section>
    </main>
  )
}
