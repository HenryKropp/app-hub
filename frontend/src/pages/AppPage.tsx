import { useEffect, useRef, useState, type ReactNode } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, CalendarClock, ExternalLink, FolderOpen, KeyRound, Play, TriangleAlert } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { HistoryTimeline } from "@/components/HistoryTimeline"
import { KindTab, LoadError, SavedIn } from "@/components/bits"
import { AccessSection } from "@/components/ficha/AccessSection"
import { BuildEditor, choiceProblems, DependsEditor, IdentityEditor, InputsEditor, NotesEditor, OutputsEditor, PeopleEditor } from "@/components/ficha/editors"
import { EditLink, Part, SaveBar, Section } from "@/components/ficha/Part"
import { BuildView, InputList, Notes, OtherApps, OutputList, People, RunList } from "@/components/ficha/views"
import { MakeShared, SharedSection } from "@/components/ficha/shared/SharedSection"
import { AppControlBar } from "@/components/ficha/AppControl"
import { ArchiveApp } from "@/components/ficha/ArchiveApp"
import { CareNav, FaceSwitch, Steps, TodoList, type Face, type MenuItem, type Step, type Todo } from "@/components/ficha/CareMenu"
import { SettingsSection, VAULT_KINDS } from "@/components/ficha/SettingsSection"
import { RunsPanel } from "@/components/ficha/RunsPanel"
import { Tech } from "@/components/ficha/shared/Tech"
import { FavoriteStar } from "@/components/FavoriteStar"
import { ScheduleDialog } from "@/components/ScheduleDialog"
import { SchedulesSection } from "@/components/SchedulesSection"
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
// The sections of the Cuidar face (ADR 029).
type CareKey = "pendencias" | "ficha" | "configuracoes" | "execucoes" | "servidor" | "acesso" | "codigo" | "historico"

const RUNS_SHOWN = 5

export function AppPage() {
  const { id } = useParams()
  const { person } = useAuth()
  const [app, setApp] = useState<AppDetail | null>(null)
  const [error, setError] = useState("")
  const [readable, setReadable] = useState(true)
  const [historyKey, setHistoryKey] = useState(0)
  const [tries, setTries] = useState(0)
  const [runs, setRuns] = useState<RunSummary[] | null>(null)
  const [scheduling, setScheduling] = useState(false)
  const [schedulesKey, setSchedulesKey] = useState(0)
  const [open, setOpen] = useState<PartKey | null>(null)
  const [draft, setDraft] = useState<Draft | null>(null)
  const [lists, setLists] = useState<Lists>({ areas: [], people: [], apps: [] })
  const listsAsked = useRef(false)
  // null until the app loads: whoever takes care of it starts on Cuidar.
  const [face, setFace] = useState<Face | null>(null)
  const [section, setSection] = useState<CareKey>("pendencias")
  // Sections already opened stay mounted (hidden), so going back keeps their state.
  const [visited, setVisited] = useState<Set<CareKey>>(() => new Set(["pendencias"]))
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

  // Another app on the same page: forget the open part and where the person was.
  useEffect(() => {
    setApp(null)
    setRuns(null)
    setOpen(null)
    setDraft(null)
    setFace(null)
    setSection("pendencias")
    setVisited(new Set(["pendencias"]))
  }, [id])

  // The link Claude gives (…/apps/N#chaves) opens straight on Configurações.
  const appId = app?.id
  useEffect(() => {
    if (appId == null || !["#chaves", "#configuracoes"].includes(window.location.hash)) return
    setFace("cuidar")
    setSection("configuracoes")
    setVisited((v) => new Set(v).add("configuracoes"))
  }, [appId])

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
    if (open && dirty && !window.confirm("Descartar o que você mudou?")) return false
    loadLists()
    setDraft({ manifest: structuredClone(app.manifest), ownerId: app.owner_id, backupId: app.backup_id ?? 0, updatedAt: app.updated_at })
    setOpen(k)
    return true
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

  const control = app.control ?? null
  const waitingApproval = !!control && control.require_approval && !control.approved
  const blocked = !!control && (control.suspended || waitingApproval)

  // Who takes care of the app gets the Cuidar face; everyone else only uses it.
  const serverPart = can.restore && (!!app.shared || m.kind === "web")
  const cares = can.edit || can.tech || serverPart || !!control?.can_manage
  const shownFace: Face = cares ? (face ?? "cuidar") : "usar"

  const go = (k: CareKey) => {
    setFace("cuidar")
    setSection(k)
    setVisited((v) => (v.has(k) ? v : new Set(v).add(k)))
  }
  const editIn = (k: CareKey, p: PartKey) => {
    if (startEdit(p)) go(k)
  }

  // Pendências: what stands between the app and the team using it with peace of mind.
  const todos: Todo[] = []
  if (!readable) {
    todos.push({ key: "pasta", title: "Não consegui ler a pasta do app", detail: "mudanças novas no código não são registradas e não dá para voltar versões" })
  }
  if (app.shared && can.restore && (app.shared.status === "draft" || app.shared.status === "crashed")) {
    todos.push({
      key: "publicar",
      title: app.shared.status === "draft" ? "Publicar no servidor" : "O app parou com erro no servidor",
      detail: app.shared.status === "draft" ? "a equipe ainda não abre pelo endereço" : "veja o que aconteceu e ligue de novo",
      action: <Button size="sm" variant="outline" onClick={() => go("servidor")}>Ir para Servidor</Button>,
    })
  }
  if (!m.backup && can.edit && can.access) {
    todos.push({ key: "reserva", title: "Escolher uma reserva", detail: "alguém que cuida do app quando o responsável não está", action: <Button size="sm" variant="outline" onClick={() => editIn("ficha", "pessoas")}>Escolher</Button> })
  }
  if (!m.summary && can.edit) {
    todos.push({ key: "resumo", title: "Escrever o resumo", detail: "uma frase sobre o que o app faz", action: <Button size="sm" variant="outline" onClick={() => startEdit("sobre")}>Escrever</Button> })
  }
  if ((m.outputs ?? []).length === 0 && can.edit) {
    todos.push({ key: "entrega", title: "Dizer o que ele entrega", detail: "quem usa sabe o que esperar", action: <Button size="sm" variant="outline" onClick={() => editIn("ficha", "entrega")}>Descrever</Button> })
  }
  // Configurações: every app that runs (ADR 032).
  const hasSettings = VAULT_KINDS.includes(m.kind) || !!app.shared
  const missing = app.missing_settings ?? []
  if (hasSettings && missing.length > 0) {
    todos.push({
      key: "configuracoes",
      title: "Preencher as configurações",
      detail: `falta: ${missing.join(", ")}`,
      action: <Button size="sm" variant="outline" onClick={() => go("configuracoes")}>Ir para Configurações</Button>,
    })
  }
  const pendingCount = todos.length + (blocked ? 1 : 0)

  const steps: Step[] = [{ label: "Registrado", done: true, state: "feito" }]
  if (control?.require_approval || control?.suspended) {
    steps.push({
      label: "Aprovado",
      done: control.approved && !control.suspended,
      bad: control.suspended,
      state: control.suspended ? "suspenso" : control.approved ? "feito" : control.can_manage ? "esperando você" : "esperando um administrador",
    })
  }
  if (app.shared) {
    steps.push({ label: "Publicado", done: app.shared.status !== "draft", state: app.shared.status === "draft" ? "rascunho" : "feito" })
  }
  steps.push({ label: "Com reserva", done: !!m.backup, state: m.backup ? m.backup.name : "ninguém ainda" })

  const menu: MenuItem[] = [
    { key: "pendencias", label: "Pendências", count: pendingCount, warn: true },
    { key: "ficha", label: "Ficha", group: "App" },
  ]
  if (hasSettings) menu.push({ key: "configuracoes", label: "Configurações", group: "App" })
  // Execuções (ADR 033): the same runs as the Usar face, with what is running on top.
  const showRuns = app.can_run || (runs?.length ?? 0) > 0
  if (showRuns) menu.push({ key: "execucoes", label: "Execuções", group: "App" })
  if (serverPart) menu.push({ key: "servidor", label: "Servidor", group: "App" })
  if (can.tech) {
    menu.push({ key: "acesso", label: "Acesso", group: "Ajustes" })
    menu.push({ key: "codigo", label: "Código", group: "Técnico", count: notes.length })
    menu.push({ key: "historico", label: "Histórico", group: "Técnico" })
  }
  const current: CareKey = menu.some((i) => i.key === section) ? section : "pendencias"
  const panel = (k: CareKey, body: ReactNode) =>
    (visited.has(k) || current === k) && (
      <div key={k} hidden={current !== k} role="region" aria-label={menu.find((i) => i.key === k)?.label}>
        {body}
      </div>
    )

  return (
    <article>
      <Link to="/apps" className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Apps
      </Link>
      <header className="flex flex-col gap-5 pb-6 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0 flex-1">
          {open === "sobre" && draft ? (
            <div className="flex max-w-2xl flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-xs" role="group" aria-label="Sobre">
              <IdentityEditor value={draft.manifest} onChange={(v) => setDraft({ ...draft, manifest: v })} areas={lists.areas} areaLocked={!can.access} />
              <SaveBar dirty={dirty} onSave={save} onCancel={close} extra={<Button size="sm" variant="outline" onClick={reload}>Recarregar</Button>} />
            </div>
          ) : (
            <>
              <h1 className="flex items-center gap-2 font-heading text-3xl leading-tight md:text-4xl">
                {m.name}
                <FavoriteStar kind="app" id={app.id} on={app.favorite} name={m.name} />
              </h1>
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
            {app.can_run && (
              <Button variant="outline" onClick={() => run(target)} disabled={busy.has(app.id)} className="gap-2">
                <Play className="size-4" /> No meu computador
              </Button>
            )}
          </div>
        )}
        {!opens && app.can_run && (
          <div className="flex shrink-0 flex-col items-start gap-2 lg:items-end">
            <Button variant="run" onClick={() => run(target)} disabled={busy.has(app.id)} className="h-11 gap-2 pr-6 pl-5 text-[0.95rem]">
              <Play className="size-4" /> {m.kind === "web" ? "Abrir no computador" : "Rodar"}
            </Button>
            {m.kind === "script" && (
              <Button variant="outline" onClick={() => setScheduling(true)} className="gap-2">
                <CalendarClock className="size-4" /> Agendar
              </Button>
            )}
            {saved && <SavedIn folder={saved} full onChange={() => run(target, { ask: true })} className="max-w-sm text-sm lg:text-right" />}
          </div>
        )}
      </header>

      {cares && (
        <div className="mb-6">
          <FaceSwitch face={shownFace} onChange={setFace} pending={pendingCount} />
        </div>
      )}

      {scheduling && (
        <ScheduleDialog appId={app.id} appName={m.name} manifest={m} onClose={() => setScheduling(false)}
          onSaved={() => { setScheduling(false); setSchedulesKey((k) => k + 1); toast.success("Agendamento salvo.") }} />
      )}

      {shownFace === "usar" ? (
        <div>
          {blocked && <AppControlBar className="mb-6" appId={app.id} control={control} onChanged={(c) => setApp({ ...app, control: c })} />}
          {!readable && !cares && (
            <p role="alert" className="mb-4 flex gap-2 text-sm text-warn">
              <TriangleAlert className="mt-0.5 size-4 shrink-0" /> Não consegui ler a pasta do app agora.
            </p>
          )}
          <Section title="O que ele pede"><InputList inputs={m.inputs ?? []} /></Section>
          <Section title="O que ele entrega"><OutputList outputs={m.outputs} /></Section>
          {(app.can_run || (runs?.length ?? 0) > 0) && (
            <Section title={can.runs ? "Execuções" : "Suas execuções"}>
              <RunList runs={runs} everyone={can.runs} />
            </Section>
          )}
          {m.kind === "script" && <SchedulesSection appId={app.id} appName={m.name} manifest={m} reloadKey={schedulesKey} />}
          {app.shared && !can.restore && <SharedSection app={app} can={can} onChanged={sharedChanged} />}
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-[11rem_minmax(0,1fr)] md:gap-10">
          <CareNav items={menu} current={current} onPick={(k) => go(k as CareKey)} />
          <div className="min-w-0">
            {panel("pendencias", (
              <div className="flex flex-col gap-4">
                <Steps steps={steps} />
                <AppControlBar className="" appId={app.id} control={control} onChanged={(c) => setApp({ ...app, control: c })} />
                {todos.length > 0 ? (
                  <TodoList todos={todos} />
                ) : (
                  !blocked && <p className="text-sm text-muted-foreground">Tudo em dia. Nada pendente neste app.</p>
                )}
              </div>
            ))}

            {panel("ficha", (
              <div className="[&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
                {part("pede", "O que ele pede", <InputList inputs={m.inputs ?? []} />, () => (
                  <InputsEditor value={draft!.manifest.inputs ?? []} onChange={(v) => setManifest({ inputs: v })} />
                ))}
                {part("entrega", "O que ele entrega", <OutputList outputs={m.outputs} />, () => (
                  <OutputsEditor value={draft!.manifest.outputs ?? []} onChange={(v) => setManifest({ outputs: v })} />
                ))}
                {part("pessoas", "Quem cuida", <People owner={m.owner?.name ?? ""} backup={m.backup?.name ?? ""} />, () => (
                  <PeopleEditor people={lists.people} ownerId={draft!.ownerId} backupId={draft!.backupId} onChange={(o, b) => setDraft({ ...draft!, ownerId: o, backupId: b })} />
                ), can.edit && can.access)}
              </div>
            ))}

            {showRuns && panel("execucoes", <RunsPanel appId={app.id} everyone={can.runs} reloadKey={signal} />)}

            {serverPart && panel("servidor", (
              <div className="[&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
                {app.shared ? <SharedSection app={app} can={can} onChanged={sharedChanged} /> : <MakeShared app={app} onChanged={setApp} />}
                {app.shared && can.tech && <Tech app={app} onChanged={sharedChanged} />}
              </div>
            ))}

            {hasSettings && panel("configuracoes", (
              <SettingsSection appId={app.id} kind={m.kind} onChanged={() => sharedChanged()} />
            ))}

            {can.tech && person && panel("acesso", (
              <div className="[&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
                <AccessSection target={{ kind: "app", id: app.id, can: app.can, areaName: app.manifest.area }} me={person} />
              </div>
            ))}

            {can.tech && panel("codigo", (
              <div className="[&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
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
                  <Section title="Chaves no código">
                    <p className="flex gap-2 text-sm">
                      <KeyRound className="mt-0.5 size-4 shrink-0 text-destructive" />
                      Usa {m.secrets!.length} chave(s) ou senha(s) escritas no código. Os valores nunca aparecem no Ziv. Guarde cada uma em Configurações.
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
                <Section title="Onde está">
                  <p className="flex items-start gap-2 text-sm">
                    <FolderOpen className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                    <span className="[overflow-wrap:anywhere]">{app.folder_display}</span>
                  </p>
                  <p className="mt-1.5 pl-6 text-xs text-muted-foreground">A ficha também fica salva nessa pasta.</p>
                </Section>
              </div>
            ))}

            {can.tech && panel("historico", (
              <HistoryTimeline preview={5} appId={app.id} canEdit={can.edit} canRestore={can.restore} reloadKey={historyKey} onChanged={() => void api.app(app.id).then(setApp, () => {})} />
            ))}
          </div>
        </div>
      )}

      {/* Arquivar (ADR 031): only the owner or an administrator. */}
      {can.access && <ArchiveApp className="mt-12" appId={app.id} name={m.name} shared={!!app.shared && app.shared.status !== "draft"} />}
    </article>
  )
}
