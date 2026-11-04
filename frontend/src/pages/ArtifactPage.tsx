import { useCallback, useEffect, useState, type FormEvent } from "react"
import { Link, useLocation, useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Download, ExternalLink, Link2 } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { ByClaude, Field, LoadError, NativeSelect, PageHeader } from "@/components/bits"
import { AccessSection } from "@/components/ficha/AccessSection"
import { EditLink, Section } from "@/components/ficha/Part"
import { ArtifactSourceField, ClaudeLinkHint, isClaudeLink, readAsBase64, sourceReady, type ArtifactSource } from "@/components/ArtifactSourceField"
import { FavoriteStar } from "@/components/FavoriteStar"
import { ApiError, api, artifactContentUrl, type Area, type ArtifactDetail, type PersonRow } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { has } from "@/lib/permissions"
import { parseTags } from "@/lib/tags"
import { cn } from "@/lib/utils"

// The artifact runs in a sandbox with no same-origin access, so it can never reach the Hub's session.
export const ARTIFACT_SANDBOX = "allow-scripts allow-popups allow-forms allow-downloads"
// An outside site that allows being framed keeps its own origin (it needs it for its login and
// storage); being another origin, it still cannot touch the Ziv page. The server never marks a
// link to the Ziv itself as embeddable.
export const LINK_SANDBOX = "allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-forms allow-downloads"

const fmtDate = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" })
const domainOf = (url: string) => {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}
const sameOrigin = (url: string) => {
  try {
    return new URL(url).origin === window.location.origin
  } catch {
    return true
  }
}
const outline = cn(buttonVariants({ variant: "outline" }), "h-9 gap-1.5")

export function ArtifactPage() {
  const { id: idParam } = useParams()
  const id = Number(idParam)
  const { person } = useAuth()
  const navigate = useNavigate()
  const { hash } = useLocation()
  const [art, setArt] = useState<ArtifactDetail | null>(null)
  const [error, setError] = useState("")
  const [notFound, setNotFound] = useState(false)

  const load = useCallback(() => {
    setError("")
    api.artifact(id).then(setArt, (e: Error) => {
      if (e instanceof ApiError && e.status === 404) setNotFound(true)
      else setError(e.message)
    })
  }, [id])
  useEffect(load, [load])

  const loaded = !!art
  useEffect(() => {
    if (loaded && hash === "#acesso") document.getElementById("acesso")?.scrollIntoView?.()
  }, [loaded, hash])

  if (notFound) {
    return (
      <>
        <PageHeader title="Artifact não encontrado." />
        <Link to="/artifacts" className={outline}>
          <ArrowLeft className="size-4" />
          Voltar aos artifacts
        </Link>
      </>
    )
  }
  if (error) return <LoadError message={error} onRetry={load} />
  if (!art) return <p className="text-muted-foreground">Carregando…</p>

  const current = art.versions.find((v) => v.number === art.current) ?? art.versions[0]
  const isFile = art.kind === "file"
  const url = current?.url ?? ""
  // Never frame the Ziv itself with allow-same-origin (the server also refuses to mark it).
  const embedLink = !isFile && !!current?.embeddable && !sameOrigin(url)

  return (
    <>
      <header className="mb-6 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <Link to="/artifacts" className="mb-2 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="size-3.5" />
            Artifacts
          </Link>
          <h1 className="flex items-center gap-2 font-heading text-2xl leading-tight md:text-[1.75rem]">
            {art.name}
            <FavoriteStar kind="artifact" id={art.id} on={art.favorite} name={art.name} />
          </h1>
          <div className="mt-2 flex flex-wrap items-center gap-1.5 text-sm text-muted-foreground">
            <span>{art.area}</span>
            {art.origin === "mcp" && <ByClaude />}
            {art.tags.map((t) => (
              <span key={t} className="rounded-full border border-border px-2 py-0.5 text-xs">
                {t}
              </span>
            ))}
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          {isFile ? (
            <>
              <a href={artifactContentUrl(art.id)} target="_blank" rel="noopener noreferrer" className={outline}>
                <ExternalLink className="size-4" />
                Abrir em tela cheia
              </a>
              <a href={artifactContentUrl(art.id, undefined, true)} className={outline}>
                <Download className="size-4" />
                Baixar
              </a>
            </>
          ) : (
            <a href={url} target="_blank" rel="noopener noreferrer" className={outline}>
              <ExternalLink className="size-4" />
              {embedLink ? "Abrir em tela cheia" : "Abrir"}
            </a>
          )}
        </div>
      </header>

      {isFile ? (
        <div className="overflow-hidden rounded-xl border border-border bg-card">
          <iframe key={art.current} title={art.name} src={artifactContentUrl(art.id, art.current)} sandbox={ARTIFACT_SANDBOX} className="block w-full min-h-[480px] h-[calc(100vh-12rem)] bg-white" />
        </div>
      ) : embedLink ? (
        <div className="overflow-hidden rounded-xl border border-border bg-card">
          <iframe key={art.current} title={art.name} src={url} sandbox={LINK_SANDBOX} referrerPolicy="no-referrer" className="block w-full min-h-[480px] h-[calc(100vh-12rem)] bg-white" />
        </div>
      ) : (
        <>
          <div className="flex items-center gap-4 rounded-xl border border-border bg-card p-5">
            <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-accent">
              <Link2 className="size-5" />
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-xs text-muted-foreground">Este artifact abre em outro site</p>
              <p className="truncate font-medium">{domainOf(url)}</p>
            </div>
            <a href={url} target="_blank" rel="noopener noreferrer" className={outline}>
              Abrir
            </a>
          </div>
          {art.can.edit && isClaudeLink(url) && <ClaudeLinkHint className="mt-3" />}
        </>
      )}

      <div className="mt-8">
        <AboutSection art={art} onSaved={setArt} />
        <VersionsSection art={art} onChanged={setArt} />
        {art.can.tech && person && (
          <div id="acesso">
            <AccessSection target={{ kind: "artifact", id: art.id, can: art.can, areaName: art.area }} me={person} />
          </div>
        )}
        {(art.can.access || has(person, "manage_roles")) && (
          <Section title="Apagar">
            <DeleteButton art={art} onDeleted={() => navigate("/artifacts")} />
          </Section>
        )}
      </div>
    </>
  )
}

function DeleteButton({ art, onDeleted }: { art: ArtifactDetail; onDeleted: () => void }) {
  const [error, setError] = useState("")
  async function remove() {
    if (!window.confirm(`Apagar "${art.name}"? Isso não pode ser desfeito.`)) return
    try {
      await api.deleteArtifact(art.id)
      onDeleted()
    } catch (e) {
      setError((e as Error).message)
    }
  }
  return (
    <div className="flex flex-col items-start gap-2">
      <Button type="button" variant="outline" className="border-destructive/40 text-destructive hover:bg-destructive/10" onClick={remove}>
        Apagar artifact
      </Button>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}

function AboutSection({ art, onSaved }: { art: ArtifactDetail; onSaved: (a: ArtifactDetail) => void }) {
  // ficha: name, description, tags (can.edit). care: owner, backup, area (can.access).
  const [editing, setEditing] = useState<"ficha" | "care" | null>(null)
  const [name, setName] = useState(art.name)
  const [summary, setSummary] = useState(art.summary)
  const [tagsText, setTagsText] = useState(art.tags.join(", "))
  const [ownerId, setOwnerId] = useState(art.owner_id)
  const [backupId, setBackupId] = useState(art.backup_id ?? 0)
  const [areaId, setAreaId] = useState(art.area_id)
  const [people, setPeople] = useState<PersonRow[] | null>(null)
  const [areas, setAreas] = useState<Area[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  function open() {
    setName(art.name)
    setSummary(art.summary)
    setTagsText(art.tags.join(", "))
    setError("")
    setEditing("ficha")
  }
  function openCare() {
    setOwnerId(art.owner_id)
    setBackupId(art.backup_id ?? 0)
    setAreaId(art.area_id)
    setError("")
    setEditing("care")
    api.people().then(setPeople, (e: Error) => setError(e.message))
    api.areas().then(setAreas, (e: Error) => setError(e.message))
  }
  async function submit(body: Parameters<typeof api.updateArtifact>[1]) {
    setBusy(true)
    setError("")
    try {
      onSaved(await api.updateArtifact(art.id, body))
      setEditing(null)
    } catch (err) {
      setError((err as Error).message)
    }
    setBusy(false)
  }
  function save(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    void submit({ name: name.trim(), summary: summary.trim(), tags: parseTags(tagsText) })
  }
  function saveCare(e: FormEvent) {
    e.preventDefault()
    if (!ownerId || !areaId) return
    // The ficha goes along unchanged: the Hub replaces name, description and tags on every save.
    void submit({ name: art.name, summary: art.summary, tags: art.tags, owner_id: ownerId, backup_id: backupId === ownerId ? 0 : backupId, area_id: areaId })
  }

  const errorLine = error && (
    <p role="alert" className="text-sm text-destructive">
      {error}
    </p>
  )
  const buttons = (ready: boolean) => (
    <div className="flex gap-2">
      <Button type="submit" disabled={busy || !ready}>
        {busy ? "Salvando…" : "Salvar"}
      </Button>
      <Button type="button" variant="outline" onClick={() => setEditing(null)}>
        Cancelar
      </Button>
    </div>
  )
  const aside = !editing && (art.can.edit || art.can.access) && (
    <div className="flex flex-col items-start gap-1">
      {art.can.edit && <EditLink part="o artifact" onClick={open} />}
      {art.can.access && (
        <button type="button" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline" onClick={openCare}>
          Editar quem cuida
        </button>
      )}
    </div>
  )

  return (
    <Section title="Sobre" id="sobre" aside={aside || undefined}>
      {editing === "ficha" ? (
        <form onSubmit={save} className="flex max-w-xl flex-col gap-4">
          <Field label="Nome" htmlFor="ed-name">
            <Input id="ed-name" value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label="Descrição" htmlFor="ed-summary">
            <Textarea id="ed-summary" value={summary} onChange={(e) => setSummary(e.target.value)} rows={2} />
          </Field>
          <Field label="Etiquetas" htmlFor="ed-tags" hint="Separe por vírgula.">
            <Input id="ed-tags" value={tagsText} onChange={(e) => setTagsText(e.target.value)} />
          </Field>
          {errorLine}
          {buttons(!!name.trim())}
        </form>
      ) : editing === "care" ? (
        <form onSubmit={saveCare} className="flex max-w-xl flex-col gap-4">
          {!people || !areas ? (
            error ? errorLine : <p className="text-sm text-muted-foreground">Carregando…</p>
          ) : (
            <>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Responsável" htmlFor="ed-owner" hint="Quem mantém o artifact e responde por ele.">
                  <NativeSelect id="ed-owner" value={ownerId} onChange={(e) => setOwnerId(Number(e.target.value))}>
                    {people.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </NativeSelect>
                </Field>
                <Field label="Reserva" htmlFor="ed-backup" hint="Quem assume se o responsável não estiver.">
                  <NativeSelect id="ed-backup" value={backupId === ownerId ? 0 : backupId} onChange={(e) => setBackupId(Number(e.target.value))}>
                    <option value={0}>Ninguém</option>
                    {people
                      .filter((p) => p.id !== ownerId)
                      .map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                  </NativeSelect>
                </Field>
              </div>
              <Field label="Área" htmlFor="ed-area" hint="O líder da área cuida do artifact junto.">
                <NativeSelect id="ed-area" className="sm:max-w-[calc(50%-0.5rem)]" value={areaId} onChange={(e) => setAreaId(Number(e.target.value))}>
                  {areas.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </NativeSelect>
              </Field>
              {errorLine}
              {buttons(!!ownerId && !!areaId)}
            </>
          )}
        </form>
      ) : (
        <dl className="flex flex-col gap-3 text-sm">
          <p>{art.summary || <span className="text-muted-foreground">Sem descrição.</span>}</p>
          <div className="flex flex-wrap gap-x-8 gap-y-2 text-muted-foreground">
            <div>
              Responsável: <span className="text-foreground">{art.owner_name}</span>
            </div>
            <div>
              Reserva: <span className="text-foreground">{art.backup_name || "ninguém"}</span>
            </div>
            <div>
              Área: <span className="text-foreground">{art.area}</span>
            </div>
          </div>
        </dl>
      )}
    </Section>
  )
}

function VersionsSection({ art, onChanged }: { art: ArtifactDetail; onChanged: (a: ArtifactDetail) => void }) {
  const [form, setForm] = useState(false)
  const [source, setSource] = useState<ArtifactSource>({ kind: "file", url: "" })
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [secrets, setSecrets] = useState(0)

  async function restore(n: number) {
    setError("")
    try {
      onChanged(await api.restoreArtifact(art.id, n))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function send(confirm: boolean) {
    setBusy(true)
    setError("")
    try {
      const body: Parameters<typeof api.artifactVersion>[1] = { kind: source.kind, note: note.trim() }
      if (source.kind === "link") body.url = source.url.trim()
      else {
        body.file_name = source.file!.name
        body.content_base64 = await readAsBase64(source.file!)
      }
      if (confirm) body.confirm_secrets = true
      onChanged(await api.artifactVersion(art.id, body))
      setForm(false)
      setSource({ kind: "file", url: "" })
      setNote("")
    } catch (e) {
      const data = e instanceof ApiError ? (e.body as { code?: string; secret_count?: number } | undefined) : undefined
      if (e instanceof ApiError && e.status === 409 && data?.code === "artifact_secrets_confirmation_required") setSecrets(data.secret_count ?? 1)
      else setError((e as Error).message)
    }
    setBusy(false)
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (sourceReady(source) && !busy) void send(false)
  }

  return (
    <Section
      title="Versões"
      id="versoes"
      aside={
        art.can.edit && !form ? (
          <Button type="button" variant="outline" size="sm" onClick={() => setForm(true)}>
            Enviar versão nova
          </Button>
        ) : undefined
      }
    >
      {form && (
        <form onSubmit={submit} className="mb-5 flex max-w-xl flex-col gap-4 rounded-xl border border-border bg-card p-4">
          <ArtifactSourceField value={source} onChange={(v) => { setSource(v); setSecrets(0) }} />
          <Field label="O que mudou?" htmlFor="ver-note">
            <Input id="ver-note" value={note} onChange={(e) => setNote(e.target.value)} />
          </Field>
          {secrets > 0 && (
            <div role="alertdialog" aria-label="Possíveis senhas" className="flex flex-col gap-3 rounded-lg border border-border bg-background p-3">
              <p className="text-sm">Encontrei {secrets} possíveis senhas ou chaves nesse arquivo. Quem abrir o artifact poderá vê-las.</p>
              <div className="flex gap-2">
                <Button type="button" disabled={busy} onClick={() => { setSecrets(0); void send(true) }}>
                  Enviar mesmo assim
                </Button>
                <Button type="button" variant="outline" onClick={() => setSecrets(0)}>
                  Cancelar
                </Button>
              </div>
            </div>
          )}
          <div className="flex gap-2">
            <Button type="submit" disabled={!sourceReady(source) || busy}>
              {busy ? "Enviando…" : "Enviar"}
            </Button>
            <Button type="button" variant="outline" onClick={() => { setForm(false); setSecrets(0) }}>
              Cancelar
            </Button>
          </div>
        </form>
      )}
      {error && (
        <p role="alert" className="mb-3 text-sm text-destructive">
          {error}
        </p>
      )}
      <ul className="flex flex-col divide-y divide-border">
        {art.versions.map((v) => {
          const isCurrent = v.number === art.current
          const href = v.kind === "link" ? v.url : artifactContentUrl(art.id, v.number)
          return (
            <li key={v.number} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 py-3 first:pt-0">
              <div className="min-w-0 text-sm">
                <p className="font-medium">
                  Versão {v.number}
                  {isCurrent && <span className="ml-2 rounded-full bg-accent px-2 py-0.5 text-xs font-normal text-muted-foreground">atual</span>}
                  {v.restored_from != null && <span className="ml-2 font-normal text-muted-foreground">(cópia da versão {v.restored_from})</span>}
                </p>
                {v.note && <p>{v.note}</p>}
                <p className="text-muted-foreground">
                  {v.author_name} · {fmtDate(v.created_at)}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <a href={href} target="_blank" rel="noopener noreferrer" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
                  Abrir
                </a>
                {art.can.restore && !isCurrent && (
                  <Button type="button" variant="outline" size="sm" onClick={() => void restore(v.number)}>
                    Voltar para esta
                  </Button>
                )}
              </div>
            </li>
          )
        })}
      </ul>
    </Section>
  )
}
