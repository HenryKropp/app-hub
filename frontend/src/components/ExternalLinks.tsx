import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react"
import { createPortal } from "react-dom"
import QRCode from "qrcode"
import { Link } from "react-router-dom"
import { toast } from "sonner"
import { Copy, Link2, Mail, Maximize2, QrCode, Share2, X } from "lucide-react"
import { Section } from "@/components/ficha/Part"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { api, type ExternalLink, type ExternalLinkInput } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { cn } from "@/lib/utils"

// Links that open an artifact for someone outside the company (ADR 045).

const EXPIRY: { days: number; label: string }[] = [
  { days: 7, label: "7 dias" },
  { days: 30, label: "30 dias" },
  { days: 90, label: "90 dias" },
  { days: 365, label: "1 ano" },
]

const STATUS: Record<ExternalLink["status"], { label: string; tone: string }> = {
  ativo: { label: "Ativo", tone: "text-ok" },
  vencido: { label: "Vencido", tone: "text-muted-foreground" },
  cancelado: { label: "Cancelado", tone: "text-muted-foreground" },
  fechado: { label: "Fechado: quem criou perdeu o acesso", tone: "text-warn" },
}

// The share window and the section on the same page are separate components;
// a link created in one tells the other to reload.
const LINKS_CHANGED = "ziv:artifact-links-changed"

const formatDate = (iso: string) => new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" })

async function copy(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    toast.success("Link copiado.")
  } catch {
    toast.error("Não deu para copiar. Selecione o link e copie.")
  }
}

export function ArtifactLinksSection({ artifactId, currentVersion }: { artifactId: number; currentVersion: number }) {
  const [data, setData] = useState<{ email_available: boolean; can_share: boolean; links: ExternalLink[] } | null>(null)
  const [error, setError] = useState("")
  const [form, setForm] = useState(false)
  const load = useCallback(() => {
    api.artifactLinks(artifactId).then(setData, (e: Error) => setError(e.message))
  }, [artifactId])
  useEffect(load, [load])
  useEffect(() => {
    const reload = (e: Event) => { if ((e as CustomEvent<number>).detail === artifactId) load() }
    window.addEventListener(LINKS_CHANGED, reload)
    return () => window.removeEventListener(LINKS_CHANGED, reload)
  }, [artifactId, load])

  if (error) return <Section title="Links externos"><p role="alert" className="text-sm text-destructive">{error}</p></Section>
  if (!data || !data.can_share) return null

  return (
    <Section
      title="Links externos"
      id="links-externos"
      aside={<p className="text-xs text-muted-foreground">Para mandar a quem é de fora da empresa, sem login no Ziv.</p>}
    >
      {form ? (
        <CreateLinkForm
          artifactId={artifactId}
          currentVersion={currentVersion}
          emailAvailable={data.email_available}
          onCancel={() => setForm(false)}
          onCreated={(l) => {
            setForm(false)
            setData({ ...data, links: [l, ...data.links] })
          }}
        />
      ) : (
        <Button type="button" variant="outline" onClick={() => setForm(true)}>
          <Link2 className="size-4" />
          Criar link para fora
        </Button>
      )}
      {data.links.length > 0 && (
        <ul className="mt-4 divide-y divide-border rounded-xl border border-border">
          {data.links.map((l) => <LinkRow key={l.id} link={l} onCancelled={load} />)}
        </ul>
      )}
    </Section>
  )
}

/** "Compartilhar" at the top of the artifact page: the same form, in a window. */
export function ShareArtifactButton({ artifactId, currentVersion }: { artifactId: number; currentVersion: number }) {
  const [info, setInfo] = useState<{ email_available: boolean; can_share: boolean } | null>(null)
  const [open, setOpen] = useState(false)
  const [created, setCreated] = useState<ExternalLink | null>(null)
  const [qr, setQr] = useState("")
  const [full, setFull] = useState(false)
  useEffect(() => {
    api.artifactLinks(artifactId).then(setInfo, () => setInfo(null))
  }, [artifactId])
  if (!info?.can_share) return null
  const change = (v: boolean) => {
    setOpen(v)
    if (!v) {
      setCreated(null)
      setQr("")
    }
  }
  return (
    <>
      <Button type="button" variant="outline" className="h-9 gap-1.5" onClick={() => setOpen(true)}>
        <Share2 className="size-4" />
        Compartilhar
      </Button>
      <Dialog open={open} onOpenChange={change}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Compartilhar fora da empresa</DialogTitle>
            <DialogDescription>Um link para quem é de fora abrir este artifact, sem login no Ziv.</DialogDescription>
          </DialogHeader>
          {created ? (
            <>
              <div className="min-w-0 space-y-2 text-sm">
                <p className="font-medium">Link criado. Copie e mande para quem vai abrir.</p>
                <code className="block w-full truncate rounded bg-muted px-2 py-1.5 text-xs" title={created.url}>{created.url}</code>
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" variant="outline" onClick={() => void copy(created.url)}>
                    <Copy className="size-3.5" />
                    Copiar link
                  </Button>
                  {!qr && (
                    <Button type="button" size="sm" variant="outline" onClick={() => void QRCode.toString(created.url, { type: "svg", margin: 1 }).then(setQr)}>
                      <QrCode className="size-3.5" />
                      QR Code
                    </Button>
                  )}
                </div>
                {qr && (
                  <div className="flex flex-col items-center gap-2 pt-2">
                    <img alt="QR Code do link" src={svgSrc(qr)} className="size-48 rounded bg-white p-2" />
                    <Button type="button" size="sm" variant="outline" onClick={() => { setOpen(false); setFull(true) }}>
                      <Maximize2 className="size-3.5" />
                      Tela cheia
                    </Button>
                  </div>
                )}
              </div>
              <DialogFooter>
                <Button type="button" onClick={() => change(false)}>Pronto</Button>
              </DialogFooter>
            </>
          ) : (
            <CreateLinkForm
              bare
              artifactId={artifactId}
              currentVersion={currentVersion}
              emailAvailable={info.email_available}
              onCancel={() => change(false)}
              onCreated={(l) => {
                setCreated(l)
                window.dispatchEvent(new CustomEvent(LINKS_CHANGED, { detail: artifactId }))
              }}
            />
          )}
        </DialogContent>
      </Dialog>
      {full && qr && <FullscreenQr svg={qr} onClose={() => { setFull(false); setOpen(true) }} />}
    </>
  )
}

const svgSrc = (svg: string) => "data:image/svg+xml;utf8," + encodeURIComponent(svg)

// The QR fills the screen so a client can scan it from a phone held up to
// a monitor or a TV. It also asks the browser for real fullscreen; if the
// browser says no, the overlay alone still covers the page.
function FullscreenQr({ svg, onClose }: { svg: string; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    const el = ref.current
    const onClose = () => close.current()
    try {
      el?.requestFullscreen?.()?.catch(() => {})
    } catch {
      // not available: the overlay is enough
    }
    const key = (e: KeyboardEvent) => e.key === "Escape" && onClose()
    const left = () => !document.fullscreenElement && onClose()
    window.addEventListener("keydown", key)
    document.addEventListener("fullscreenchange", left)
    el?.focus()
    return () => {
      window.removeEventListener("keydown", key)
      document.removeEventListener("fullscreenchange", left)
      if (document.fullscreenElement) void document.exitFullscreen?.().catch(() => {})
    }
  }, [])
  return createPortal(
    <div ref={ref} tabIndex={-1} role="dialog" aria-modal="true" aria-label="QR Code em tela cheia"
      className="fixed inset-0 z-[100] flex flex-col items-center justify-center gap-6 bg-white p-6 text-black outline-none">
      <img alt="QR Code do link" src={svgSrc(svg)} className="aspect-square w-[min(80vw,75vh)]" />
      <p className="text-lg">Aponte a câmera do celular para abrir</p>
      <button type="button" onClick={onClose} aria-label="Fechar tela cheia"
        className="absolute top-4 right-4 inline-flex items-center gap-1.5 rounded-lg border border-black/20 px-3 py-2 text-sm font-medium hover:bg-black/5">
        <X className="size-4" />
        Fechar
      </button>
    </div>,
    document.body,
  )
}

function CreateLinkForm({ artifactId, currentVersion, emailAvailable, onCancel, onCreated, bare }: {
  artifactId: number
  currentVersion: number
  emailAvailable: boolean
  onCancel: () => void
  onCreated: (l: ExternalLink) => void
  bare?: boolean
}) {
  const [kind, setKind] = useState<ExternalLinkInput["kind"]>(emailAvailable ? "email" : "open")
  const [email, setEmail] = useState("")
  const [content, setContent] = useState<ExternalLinkInput["content"]>("latest")
  const [days, setDays] = useState(30)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError("")
    try {
      onCreated(await api.createArtifactLink(artifactId, { kind, email: kind === "email" ? email : "", content, version: currentVersion, expires_days: days }))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className={cn("space-y-5", !bare && "rounded-xl border border-border p-4")}>
      <fieldset className="space-y-2">
        <legend className="mb-1 text-sm font-medium">Quem pode abrir</legend>
        <Choice name="kind" checked={kind === "email"} disabled={!emailAvailable} onChange={() => setKind("email")}
          label={<span className="inline-flex items-center gap-1.5"><Mail className="size-3.5" />Código por e-mail</span>}
          hint={emailAvailable ? "Só a pessoa do e-mail abre: ela recebe um código de 6 dígitos." : "Disponível em breve"} />
        <Choice name="kind" checked={kind === "open"} onChange={() => setKind("open")}
          label={<span className="inline-flex items-center gap-1.5"><Link2 className="size-3.5" />Link aberto</span>}
          hint="Qualquer pessoa com este link consegue abrir." />
      </fieldset>
      {kind === "email" && (
        <div className="space-y-1.5">
          <label htmlFor="link-email" className="block text-sm font-medium">E-mail de quem vai receber</label>
          <Input id="link-email" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nome@cliente.com" />
        </div>
      )}
      <fieldset className="space-y-2">
        <legend className="mb-1 text-sm font-medium">Conteúdo</legend>
        <Choice name="content" checked={content === "latest"} onChange={() => setContent("latest")} label="Sempre a versão mais recente" hint="Quando sair versão nova, o link já mostra ela." />
        <Choice name="content" checked={content === "version"} onChange={() => setContent("version")} label={`Só a versão ${currentVersion}`} hint="O link fica congelado nesta versão." />
      </fieldset>
      <div className="space-y-1.5">
        <label htmlFor="link-expiry" className="block text-sm font-medium">Prazo</label>
        <select id="link-expiry" value={days} onChange={(e) => setDays(Number(e.target.value))} className="h-10 w-full max-w-48 rounded-md border border-input bg-background px-3 text-sm">
          {EXPIRY.map((x) => <option key={x.days} value={x.days}>{x.label}</option>)}
        </select>
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex gap-2">
        <Button type="submit" disabled={busy}>{busy ? "Criando…" : "Criar link"}</Button>
        <Button type="button" variant="ghost" onClick={onCancel}>{bare ? "Cancelar" : "Voltar"}</Button>
      </div>
    </form>
  )
}

function Choice({ name, checked, disabled, onChange, label, hint }: { name: string; checked: boolean; disabled?: boolean; onChange: () => void; label: ReactNode; hint: string }) {
  return (
    <label className={cn("flex items-start gap-2.5 text-sm", disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer")}>
      <input type="radio" name={name} className="mt-1 accent-foreground" checked={checked} disabled={disabled} onChange={onChange} />
      <span>
        <span className="block font-medium">{label}</span>
        <span className="block text-xs text-muted-foreground">{hint}</span>
      </span>
    </label>
  )
}

function LinkRow({ link, onCancelled, showArtifact }: { link: ExternalLink; onCancelled: () => void; showArtifact?: boolean }) {
  const [error, setError] = useState("")
  const active = link.status === "ativo"
  const st = STATUS[link.status]
  async function cancel() {
    if (!window.confirm("Cancelar este link? Quem tiver o link não vai mais conseguir abrir.")) return
    try {
      await api.cancelLink(link.id)
      onCancelled()
    } catch (e) {
      setError((e as Error).message)
    }
  }
  return (
    <li className="space-y-2 p-4 text-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-0.5">
          {showArtifact && link.artifact_name && (
            <Link to={`/artifacts/${link.artifact_id}`} className="block font-medium hover:underline">{link.artifact_name}</Link>
          )}
          <p className={showArtifact ? "text-muted-foreground" : "font-medium"}>{link.kind === "email" ? link.recipient : "Link aberto"}</p>
          <p className="text-xs text-muted-foreground">
            {link.content === "latest" ? "Sempre a mais recente" : `Versão ${link.version_number}`} · por {link.created_by_name || "pessoa removida"} · {active ? "vence em" : link.status === "vencido" ? "venceu em" : "prazo era"} {formatDate(link.expires_at)}
          </p>
          <p className="text-xs text-muted-foreground">
            {link.opens === 0 ? "Ainda não foi aberto" : `${link.opens} ${link.opens === 1 ? "abertura" : "aberturas"}${link.last_opened_at ? `, a última ${formatWhen(link.last_opened_at)}` : ""}`}
          </p>
        </div>
        <span className={cn("shrink-0 text-xs font-medium", st.tone)}>{st.label}</span>
      </div>
      {active && (
        <div className="flex flex-wrap items-center gap-2">
          <code className="min-w-0 max-w-full truncate rounded bg-muted px-2 py-1 text-xs">{link.url}</code>
          <Button type="button" size="sm" variant="outline" onClick={() => void copy(link.url)}>
            <Copy className="size-3.5" />
            Copiar link
          </Button>
          <Button type="button" size="sm" variant="ghost" className="text-destructive hover:bg-destructive/10" onClick={() => void cancel()}>
            Cancelar
          </Button>
        </div>
      )}
      {error && <p role="alert" className="text-destructive">{error}</p>}
    </li>
  )
}

/** Admin: every link the company sent out, with who created it and how often it opened. */
export function CompanyLinksPanel() {
  const [links, setLinks] = useState<ExternalLink[] | null>(null)
  const [error, setError] = useState("")
  const load = useCallback(() => {
    api.companyLinks().then((d) => setLinks(d.links), (e: Error) => setError(e.message))
  }, [])
  useEffect(load, [load])
  return (
    <section aria-labelledby="links-externos" className="mb-10">
      <h2 id="links-externos" className="font-heading text-xl">Links externos</h2>
      <p className="mb-3 mt-1 text-sm text-muted-foreground">Tudo o que a empresa mandou para fora por link, sem login no Ziv.</p>
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p>
        : !links ? <div className="h-16 animate-pulse rounded-xl bg-muted" />
        : links.length === 0 ? <p className="text-sm text-muted-foreground">Nenhum link criado ainda.</p>
        : (
          <ul className="divide-y divide-border rounded-xl border border-border">
            {links.map((l) => <LinkRow key={l.id} link={l} onCancelled={load} showArtifact />)}
          </ul>
        )}
    </section>
  )
}
