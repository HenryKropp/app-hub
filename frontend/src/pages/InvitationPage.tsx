import { useEffect, useState, type FormEvent } from "react"
import { ArrowLeft, ArrowRight, CircleAlert, ShieldCheck } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { AuthLayout, authBack, authError, authLink } from "@/components/AuthLayout"
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
      setError("Escolha uma senha com pelo menos 8 caracteres e que não seja muito longa.")
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
    <AuthLayout
      back={<a className={authBack} href="/acesso"><ArrowLeft size={15} /> Acessar outro espaço</a>}
      logo={<a href="/boas-vindas" aria-label="Ziv, início" className="rounded-md"><Wordmark className="h-10 w-auto" /></a>}
      below={<p>Já tem acesso? <a href="/acesso" className={authLink}>Entrar</a></p>}
    >
      {loading ? (
        <div role="status" aria-label="Validando convite" className="space-y-3">
          <div className="h-7 w-2/3 animate-pulse rounded bg-muted" />
          <div className="h-4 w-full animate-pulse rounded bg-muted" />
          <div className="h-9 w-full animate-pulse rounded bg-muted" />
          <div className="h-9 w-full animate-pulse rounded bg-muted" />
        </div>
      ) : loadError ? (
        <>
          <h1 className="font-heading text-2xl leading-tight">Não foi possível abrir o convite</h1>
          <p role="alert" className="mt-3 flex gap-2 text-sm leading-relaxed text-destructive"><CircleAlert className="mt-0.5 size-4 shrink-0" />{loadError}</p>
          {token && <Button variant="outline" size="lg" className="mt-5 w-full" onClick={() => { setPreview(null); setLoading(true); setLoadError(""); setAttempt((value) => value + 1) }}>Tentar novamente</Button>}
          <p className="mt-5 text-sm leading-relaxed text-muted-foreground">Peça à administração da empresa para confirmar se o convite ainda está ativo.</p>
        </>
      ) : preview ? (
        <>
          <h1 className="font-heading text-2xl leading-tight text-balance">Entre no espaço {preview.company_name}</h1>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">O convite foi criado para <span className="font-medium text-foreground">{preview.email}</span>. Defina seu nome e uma senha para ativar o acesso.</p>
          <form onSubmit={accept} className="mt-6 flex flex-col gap-4">
            <Field label="Seu nome" htmlFor="invite-name">
              <Input id="invite-name" value={name} onChange={(event) => setName(event.target.value)} autoComplete="name" required className="h-9" />
            </Field>
            <Field label="Crie uma senha" htmlFor="invite-password" hint="Pelo menos 8 caracteres.">
              <Input id="invite-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" minLength={8} required className="h-9" />
            </Field>
            {error && <p role="alert" className={authError}>{error}</p>}
            <Button type="submit" size="lg" className="mt-1 w-full" disabled={busy}>{busy ? "Ativando acesso…" : "Aceitar convite"} {!busy && <ArrowRight data-icon="inline-end" />}</Button>
          </form>
          <p className="mt-5 flex items-start gap-1.5 text-xs leading-relaxed text-muted-foreground"><ShieldCheck className="mt-0.5 size-3.5 shrink-0" /> Este convite é pessoal e pode ser usado uma única vez.</p>
        </>
      ) : null}
    </AuthLayout>
  )
}
