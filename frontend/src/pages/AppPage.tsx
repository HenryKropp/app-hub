import { useEffect, useRef, useState, type ReactNode } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, ExternalLink, FolderOpen, KeyRound, Play, TriangleAlert } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { HistoryTimeline } from "@/components/HistoryTimeline"
import { KindTab, LoadError, SavedIn } from "@/components/bits"
import { AccessSection } from "@/components/ficha/AccessSection"
import { BuildEditor, choiceProblems, DependsEditor, IdentityEditor, InputsEditor, NotesEditor, OutputsEditor, PeopleEditor } from "@/components/ficha/editors"
import { EditLink, Part, SaveBar, Section } from "@/components/ficha/Part"
import { BuildView, InputList, Notes, OtherApps, OutputList, People, RunList } from "@/components/ficha/views"
import { MakeShared, SharedSection } from "@/components/ficha/shared/SharedSection"
import { Tech } from "@/components/ficha/shared/Tech"
import { api, openSharedUrl, type AppDetail, type AppSummary, type Area, type Manifest, type PersonRow, type RunSummary } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useRuns } from "@/lib/runs-context"
import { formatWhen } from "@/lib/runs"
import { kindInfo } from "@/lib/kinds"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

type PartKey = "sobre" | "pede" | "entrega" | "pessoas" | "notas" | "outros" | "codigo"
// updatedAt is the version the draft started from: saving it after the app
// changed elsewhere gets the 409 instead of reverting the other parts.
type Draft = { manifest: Manifest; ownerId: number; backupId: number; updatedAt: string }
type Lists = { areas: Area[]; people: PersonRow[]; apps: AppSummary[] }

const RUNS_SHOWN = 5
const techKey = (id?: string) => `apphub-tech-open-${id}`
function readTechOpen(id?: string) {
  try {
    return localStorage.getItem(techKey(id)) === "1"
  } catch {
    return false
  }
}

export function AppPage() {
  const { id } = useParams()
  const { person } = useAuth()
  const [app, setApp] = useState<AppDetail | null>(null)
  const [error, setError] = useState("")
  const [readable, setReadable] = useState(true)
  const [historyKey, setHistoryKey] = useState(0)
  const [tries, setTries] = useState(0)
  const [runs, setRuns] = useState<RunSummary[] | null>(null)
  const [open, setOpen] = useState<PartKey | null>(null)
  const [draft, setDraft] = useState<Draft | null>(null)
  const [lists, setLists] = useState<Lists>({ areas: [], people: [], apps: [] })
  const listsAsked = useRef(false)
  const [techOpen, setTechOpen] = useState(() => readTechOpen(id))
  const [techSeen, setTechSeen] = useState(techOpen)
  const { run, busy, rememberedFor, active } = useRuns()

  // The ficha shows what the Hub already knows; the folder check (OneDrive can
  // take seconds) runs behind it and only adds a new version or the warning.
  useEffect(() => {
    if (!id) return
    let gone = false
    api.app(id).then(
      (d) => {
        if (gone) return
        setApp(d)
        setReadable(true)
        api.checkApp(id).then(
          (c) => {
            if (gone) return
            setReadable(c.folder_readable)
            if (c.version && c.version.number !== d.version?.number) {
              setApp((a) => (a ? { ...a, version: c.version } : a))
              setHistoryKey((k) => k + 1)
            }
          },
          () => !gone && setReadable(false),
        )
      },
      (e: Error) => !gone && setError(e.message),
    )
    return () => {
      gone = true
    }
  }, [id, tries])

  // Another app on the same page: forget the open part and the technical block.
  useEffect(() => {
    setApp(null)
    setRuns(null)
    setOpen(null)
    setDraft(null)
    const o = readTechOpen(id)
    setTechOpen(o)
    setTechSeen(o)
  }, [id])

  // Runs reload when one of this app's runs changes situation in the strip.
  const signal = active.filter((r) => String(r.app_id) === id).map((r) => `${r.id}:${r.status}`).join(",")
  useEffect(() => {
    if (!id) return
    let gone = false
    api.appRuns(id).then(
      (r) => !gone && setRuns(r.slice(0, RUNS_SHOWN)),
      () => !gone && setRuns([]),
    )
    return () => {
      gone = true
    }
  }, [id, tries, signal])

  if (error) return <LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} />
  if (!app) return <p className="text-muted-foreground">Carregando…</p>
  const m = app.manifest
  const can = app.can
  const target = { id: app.id, name: m.name, folder_display: app.folder_display, remembered_folder: app.remembered_folder, kind: m.kind }
  const saved = rememberedFor(target)

  const dirty = !!draft && (JSON.stringify(draft.manifest) !== JSON.stringify(app.manifest) || draft.ownerId !== app.owner_id || draft.backupId !== (app.backup_id ?? 0))

  // Areas, people and apps feed the editors; they load the first time someone edits.
  const loadLists = () => {
    if (listsAsked.current) return
    listsAsked.current = true
    let failed = false
    const or = <T,>(p: Promise<T[]>) =>
      p.catch((): T[] => {
        failed = true
        return []
      })
    Promise.all([or(api.areas()), or(api.people()), or(api.apps())]).then(([areas, people, apps]) => {
      setLists({ areas, people, apps })
      // A later "Editar" tries again.
      if (failed) listsAsked.current = false
    })
  }
  const startEdit = (k: PartKey) => {
    if (open && dirty && !window.confirm("Descartar o que você mudou?")) return
    loadLists()
    setDraft({ manifest: structuredClone(app.manifest), ownerId: app.owner_id, backupId: app.backup_id ?? 0, updatedAt: app.updated_at })
    setOpen(k)
  }
  const close = () => {
    setOpen(null)
    setDraft(null)
  }
  const save = async () => {
    const problems = choiceProblems(draft!.manifest.inputs ?? [])
    if (problems.length) throw new Error(`Falta definir ${problems.join(", ")}.`)
    const next = await api.updateApp(app.id, {
      owner_id: draft!.ownerId,
      backup_id: draft!.backupId || null,
      manifest: draft!.manifest,
      updated_at: draft!.updatedAt,
    })
    setApp(next)
    close()
    setHistoryKey((k) => k + 1)
  }
  const reload = () => {
    close()
    setTries((n) => n + 1)
  }
  const setManifest = (patch: Partial<Manifest>) => setDraft((d) => (d ? { ...d, manifest: { ...d.manifest, ...patch } } : d))
  const toggleTech = (o: boolean) => {
    setTechOpen(o)
    if (o) setTechSeen(true)
    try {
      localStorage.setItem(techKey(id), o ? "1" : "0")
    } catch {
      // Private windows may refuse storage; the block still opens.
    }
  }

  // One part of the ficha: everything a Part needs that is the same for all.
  const part = (k: PartKey, title: string, view: ReactNode, edit: () => ReactNode, canEdit = can.edit) => (
    <Part
      title={title}
      canEdit={canEdit}
      dirty={open === k && dirty}
      editing={open === k}
      onEdit={() => startEdit(k)}
      onSave={save}
      onCancel={close}
      view={view}
      edit={open === k && draft ? edit() : null}
      extra={<Button size="sm" variant="outline" onClick={reload}>Recarregar</Button>}
    />
  )

  const uses = app.depends_on_apps ?? []
  const usedBy = app.used_by ?? []
  const showOthers = uses.length > 0 || usedBy.length > 0 || can.edit
  const showBuild = !!m.build || m.kind === "executable" || m.kind === "desktop"
  const deps = m.runtime.dependencies ?? []
  const howRuns = m.run.command || m.run.url
  const notes = m.notes ?? []
  // A published shared app is opened at its address, not run on a computer.
  const opens = app.shared && app.shared.status !== "draft" ? app.shared : null
  const sharedChanged = (next?: AppDetail) => {
    if (next) setApp(next)
    else void api.app(app.id).then(setApp, () => toast.error("Não consegui atualizar a ficha. Recarregue a página para ver o estado novo."))
  }

  return (
    <article>
      <Link to="/" className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Catálogo
      </Link>
      <header className="flex flex-col gap-5 pb-8 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0 flex-1">
          {open === "sobre" && draft ? (
            <div className="flex max-w-2xl flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-xs" role="group" aria-label="Sobre">
              <IdentityEditor value={draft.manifest} onChange={(v) => setDraft({ ...draft, manifest: v })} areas={lists.areas} areaLocked={!can.access} />
              <SaveBar dirty={dirty} onSave={save} onCancel={close} extra={<Button size="sm" variant="outline" onClick={reload}>Recarregar</Button>} />
            </div>
          ) : (
            <>
              <h1 className="font-heading text-3xl leading-tight md:text-4xl">{m.name}</h1>
              <p className="mt-3 max-w-[68ch] text-base leading-relaxed text-muted-foreground">
                {m.summary || "Sem resumo ainda."}
                {can.edit && (
                  <>
                    {" "}
                    <EditLink part="Sobre" onClick={() => startEdit("sobre")} />
                  </>
                )}
              </p>
            </>
          )}
          <p className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <span title={kindInfo(m.kind).label}><KindTab kind={m.kind} /></span>
            {[
              m.area,
              m.owner && <>Responsável <span className="text-foreground">{m.owner.name}</span></>,
              m.backup ? <>Reserva <span className="text-foreground">{m.backup.name}</span></> : <span className="text-warn">Sem reserva</span>,
              app.version && <span className="inline-flex gap-1"><span>versão {app.version.number}</span><span>de {formatWhen(app.version.created_at)}</span></span>,
            ].filter(Boolean).map((p, i) => (
              <span key={i} className="inline-flex items-center gap-2"><span aria-hidden className="text-muted-foreground/50">·</span>{p}</span>
            ))}
          </p>
        </div>
        {opens && (
          <div className="flex shrink-0 flex-col items-start gap-2 lg:items-end">
            <a href={openSharedUrl(opens.slug)} target="_blank" rel="noopener" className={cn(buttonVariants({ variant: "run" }), "h-11 gap-2 pr-6 pl-5 text-[0.95rem] [&_svg]:fill-none")}>
              <ExternalLink className="size-4" /> Abrir
            </a>
          </div>
        )}
        {!opens && app.can_run && (
          <div className="flex shrink-0 flex-col items-start gap-2 lg:items-end">
            <Button variant="run" onClick={() => run(target)} disabled={busy.has(app.id)} className="h-11 gap-2 pr-6 pl-5 text-[0.95rem]">
              <Play className="size-4" /> Rodar
            </Button>
            {saved && <SavedIn folder={saved} full onChange={() => run(target, { ask: true })} className="max-w-sm text-sm lg:text-right" />}
          </div>
        )}
      </header>

      {!readable && (
        <p role="alert" className="mt-4 flex gap-2 text-sm text-warn">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" /> Não consegui ler a pasta do app agora. O histórico continua aqui, mas mudanças novas no código não são registradas e não dá para voltar versões.
        </p>
      )}

      <div className="mt-8">
        {part("pede", "O que ele pede", <InputList inputs={m.inputs ?? []} />, () => (
          <InputsEditor value={draft!.manifest.inputs ?? []} onChange={(v) => setManifest({ inputs: v })} />
        ))}
        {part("entrega", "O que ele entrega", <OutputList outputs={m.outputs} />, () => (
          <OutputsEditor value={draft!.manifest.outputs ?? []} onChange={(v) => setManifest({ outputs: v })} />
        ))}
        {(app.can_run || (runs?.length ?? 0) > 0) && (
          <Section title={can.runs ? "Execuções" : "Suas execuções"}>
            <RunList runs={runs} everyone={can.runs} />
          </Section>
        )}
        {app.shared ? (
          <SharedSection app={app} can={can} onChanged={sharedChanged} />
        ) : (
          m.kind === "web" && can.restore && <MakeShared app={app} onChanged={setApp} />
        )}
        {part("pessoas", "Quem cuida", <People owner={m.owner?.name ?? ""} backup={m.backup?.name ?? ""} />, () => (
          <PeopleEditor people={lists.people} ownerId={draft!.ownerId} backupId={draft!.backupId} onChange={(o, b) => setDraft({ ...draft!, ownerId: o, backupId: b })} />
        ), can.edit && can.access)}

        {can.tech && (
          <details open={techOpen} onToggle={(e) => toggleTech(e.currentTarget.open)} className="group/tech border-t border-border">
            <summary className="flex cursor-pointer list-none items-center gap-2.5 py-5 text-sm font-semibold [&::-webkit-details-marker]:hidden">
              <span aria-hidden className="text-lg leading-none text-muted-foreground transition-transform group-open/tech:rotate-90">›</span>
              Para quem cuida do app
              <small className="font-normal text-muted-foreground">
                {notes.length > 0 && `${notes.length} ${notes.length === 1 ? "ponto" : "pontos"} de atenção · `}código, outros apps, histórico, acesso
              </small>
            </summary>
            {techSeen && app.shared && <Tech app={app} onChanged={sharedChanged} />}
            {part("notas", "Pontos de atenção", <Notes notes={notes} />, () => (
              <NotesEditor value={draft!.manifest.notes ?? []} onChange={(v) => setManifest({ notes: v })} />
            ))}
            {showOthers && part("outros", "Outros apps", <OtherApps uses={uses} usedBy={usedBy} />, () => (
              <DependsEditor value={draft!.manifest.depends_on ?? []} onChange={(v) => setManifest({ depends_on: v })} apps={lists.apps} selfSlug={m.id} />
            ))}
            {showBuild && part("codigo", "Código-fonte", <BuildView build={m.build ?? null} />, () => (
              <BuildEditor value={draft!.manifest.build ?? null} onChange={(v) => setManifest({ build: v })} />
            ))}
            {(m.secrets ?? []).length > 0 && (
              <Section title="Chaves e senhas">
                <p className="flex gap-2 text-sm">
                  <KeyRound className="mt-0.5 size-4 shrink-0 text-destructive" />
                  Usa {m.secrets!.length} chave(s) ou senha(s) escritas no código. Os valores nunca aparecem no Kova.
                </p>
              </Section>
            )}
            {deps.length > 0 && (
              <Section title="Bibliotecas">
                <p className="flex flex-wrap gap-1.5">
                  {deps.map((d) => <code key={d} className="rounded-sm bg-secondary px-1.5 py-0.5 font-mono text-xs">{d}</code>)}
                </p>
              </Section>
            )}
            {howRuns && (
              <Section title="Como roda">
                <code className="font-mono text-xs [overflow-wrap:anywhere]">{howRuns}</code>
              </Section>
            )}
            <Section title="Histórico">
              <HistoryTimeline preview={5} appId={app.id} canEdit={can.edit} canRestore={can.restore} reloadKey={historyKey} onChanged={() => void api.app(app.id).then(setApp, () => {})} />
            </Section>
            <Section title="Onde está">
              <p className="flex items-start gap-2 text-sm">
                <FolderOpen className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                <span className="[overflow-wrap:anywhere]">{app.folder_display}</span>
              </p>
              <p className="mt-1.5 pl-6 text-xs text-muted-foreground">A ficha também fica salva nessa pasta.</p>
            </Section>
            {techSeen && person && <AccessSection app={app} me={person} />}
          </details>
        )}
      </div>
    </article>
  )
}
