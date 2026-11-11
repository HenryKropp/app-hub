import { useEffect, useRef, useState, type ReactNode } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, CalendarClock, ChevronDown, ExternalLink, FolderOpen, KeyRound, Play, TriangleAlert } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { HistoryTimeline } from "@/components/HistoryTimeline"
import { KindTab, LoadError, SavedIn } from "@/components/bits"
import { AccessSection } from "@/components/ficha/AccessSection"
import { BuildEditor, choiceProblems, DependsEditor, IdentityEditor, InputsEditor, NotesEditor, OutputsEditor, PeopleEditor } from "@/components/ficha/editors"
import { EditLink, Part, SaveBar, Section } from "@/components/ficha/Part"
import { BuildView, InputList, Notes, OtherApps, OutputList, People, RunList } from "@/components/ficha/views"
import { MakeShared, SharedSection } from "@/components/ficha/shared/SharedSection"
import { AppControlBar } from "@/components/ficha/AppControl"
import { ArchiveApp } from "@/components/ficha/ArchiveApp"
import { TodoList, type Todo } from "@/components/ficha/TodoList"
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
// The tabs of the app page (ADR 039).
type TabKey = "inicio" | "execucoes" | "documentacao" | "configuracoes" | "servidor" | "historico"

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
  const [tab, setTab] = useState<TabKey>("inicio")
  // Tabs already opened stay mounted (hidden), so going back keeps their state.
  const [visited, setVisited] = useState<Set<TabKey>>(() => new Set(["inicio"]))
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
    setTab("inicio")
    setVisited(new Set(["inicio"]))
  }, [id])

  // The link Claude gives (…/apps/N#chaves) opens straight on Configurações.
  const appId = app?.id
  useEffect(() => {
    if (appId == null || !["#chaves", "#configuracoes"].includes(window.location.hash)) return
    setTab("configuracoes")
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

  // Who takes care of the app sees the tabs and buttons of caring; everyone else only uses it.
  const serverPart = can.restore && (!!app.shared || m.kind === "web")
  const cares = can.edit || can.tech || serverPart || !!control?.can_manage
  const go = (k: TabKey) => {
    setTab(k)
    setVisited((v) => (v.has(k) ? v : new Set(v).add(k)))
  }
  const editIn = (p: PartKey) => {
    if (startEdit(p)) go("documentacao")
  }

  // Para resolver: what stands between the app and the team using it with peace of mind.
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
    todos.push({ key: "reserva", title: "Escolher uma reserva", detail: "alguém que cuida do app quando o responsável não está", action: <Button size="sm" variant="outline" onClick={() => editIn("pessoas")}>Escolher</Button> })
  }
  if (!m.summary && can.edit) {
    todos.push({ key: "resumo", title: "Escrever o resumo", detail: "uma frase sobre o que o app faz", action: <Button size="sm" variant="outline" onClick={() => editIn("sobre")}>Escrever</Button> })
  }
  if ((m.outputs ?? []).length === 0 && can.edit) {
    todos.push({ key: "entrega", title: "Dizer o que ele entrega", detail: "quem usa sabe o que esperar", action: <Button size="sm" variant="outline" onClick={() => editIn("entrega")}>Descrever</Button> })
  }
  // Configurações: every app that runs (ADR 032).
  const hasSettings = VAULT_KINDS.includes(m.kind) || !!app.shared
  const missing = app.missing_settings ?? []
  if (cares && hasSettings && missing.length > 0) {
    todos.push({
      key: "configuracoes",
      title: "Preencher as configurações",
      detail: `falta: ${missing.join(", ")}`,
      action: <Button size="sm" variant="outline" onClick={() => go("configuracoes")}>Ir para Configurações</Button>,
    })
  }
  const pendingCount = cares ? todos.length : 0

  // The tabs (ADR 039). Each thing has one place; who only uses the app sees fewer tabs.
  const showRuns = app.can_run || (runs?.length ?? 0) > 0
  const showSettings = cares && (hasSettings || can.tech || can.access || !!control?.can_manage)
  const tabs: { key: TabKey; label: string; count?: number }[] = [{ key: "inicio", label: "Início", count: pendingCount }]
  if (showRuns) tabs.push({ key: "execucoes", label: "Execuções" })
  tabs.push({ key: "documentacao", label: "Documentação" })
  if (showSettings) tabs.push({ key: "configuracoes", label: "Configurações" })
  if (serverPart) tabs.push({ key: "servidor", label: "Servidor" })
  if (can.tech) tabs.push({ key: "historico", label: "Histórico" })
  const current: TabKey = tabs.some((t) => t.key === tab) ? tab : "inicio"
  const panel = (k: TabKey, body: ReactNode) =>
    (visited.has(k) || current === k) && (
      <div key={k} id={`aba-${k}`} hidden={current !== k} role="tabpanel" aria-label={tabs.find((t) => t.key === k)?.label}>
        {body}
      </div>
    )

  // The state next to the name: what needs doing first, then whether it is up.
  const status = (() => {
    if (control?.suspended) return { tone: "bad" as const, label: "Suspenso", to: "inicio" as TabKey }
    if (waitingApproval) return { tone: "warn" as const, label: "Esperando aprovação", to: "inicio" as TabKey }
    if (pendingCount > 0) return { tone: "warn" as const, label: pendingCount === 1 ? "1 coisa para resolver" : `${pendingCount} coisas para resolver`, to: "inicio" as TabKey }
    if (!app.shared) return null
    const to: TabKey = serverPart ? "servidor" : "inicio"
    switch (app.shared.status) {
      case "draft": return { tone: "quiet" as const, label: "Rascunho", to }
      case "crashed": return { tone: "bad" as const, label: "Parou com erro", to }
      case "stopped": return { tone: "quiet" as const, label: "Desligado", to }
      case "starting": return { tone: "quiet" as const, label: "Ligando", to }
      default: return { tone: "ok" as const, label: "No ar", to }
    }
  })()

  const missingDocs = [!m.summary && "resumo", (m.outputs ?? []).length === 0 && "o que ele entrega", !m.backup && "reserva"].filter(Boolean) as string[]
  const testLocal = !!opens && app.can_run && cares

  return (
    <article>
      <Link to="/apps" className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Apps
      </Link>
      <header className="flex flex-col gap-5 pb-2 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <h1 className="flex items-center gap-2 font-heading text-3xl leading-tight md:text-4xl">
              {m.name}
              <FavoriteStar kind="app" id={app.id} on={app.favorite} name={m.name} />
            </h1>
            {status && (
              <button
                type="button"
                onClick={() => go(status.to)}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
                  status.tone === "ok" && "bg-ok/10 text-ok",
                  status.tone === "warn" && "border border-warn/35 bg-warn-soft text-warn",
                  status.tone === "bad" && "bg-destructive/10 text-destructive",
                  status.tone === "quiet" && "bg-secondary text-muted-foreground",
                )}
              >
                <span aria-hidden className="size-1.5 rounded-full bg-current" />
                {status.label}
              </button>
            )}
          </div>
          <p className="mt-3 max-w-[68ch] text-base leading-relaxed text-muted-foreground">
            {m.summary || "Sem resumo ainda."}
            {can.edit && (
              <>
                {" "}
                <EditLink part="Sobre" onClick={() => editIn("sobre")} />
              </>
            )}
          </p>
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
        {(opens || app.can_run) && (
          <div className="flex shrink-0 flex-col items-start gap-2 lg:items-end">
            <div className="flex flex-wrap items-center gap-2">
              {opens ? (
                <a href={openSharedUrl(opens.slug)} target="_blank" rel="noopener" className={cn(buttonVariants({ variant: "run" }), "h-11 gap-2 pr-6 pl-5 text-[0.95rem] [&_svg]:fill-none")}>
                  <ExternalLink className="size-4" /> Abrir
                </a>
              ) : (
                <Button variant="run" onClick={() => run(target)} disabled={busy.has(app.id)} className="h-11 gap-2 pr-6 pl-5 text-[0.95rem]">
                  <Play className="size-4" /> {m.kind === "web" ? "Abrir" : "Rodar"}
                </Button>
              )}
              {!opens && m.kind === "script" && (
                <Button variant="outline" onClick={() => setScheduling(true)} className="h-11 gap-2">
                  <CalendarClock className="size-4" /> Agendar
                </Button>
              )}
              {testLocal && (
                <DropdownMenu>
                  <DropdownMenuTrigger className={cn(buttonVariants({ variant: "outline" }), "h-11 gap-1.5")}>
                    Mais <ChevronDown className="size-4" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-72">
                    <DropdownMenuItem onClick={() => run(target)} disabled={busy.has(app.id)} className="flex-col items-start gap-0.5">
                      <span className="flex items-center gap-2"><Play className="size-4" /> Testar no meu computador</span>
                      <span className="pl-6 text-xs text-muted-foreground">Roda a cópia da sua pasta, para testar antes de publicar.</span>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
            {!opens && saved && <SavedIn folder={saved} full onChange={() => run(target, { ask: true })} className="max-w-sm text-sm lg:text-right" />}
          </div>
        )}
      </header>

      <div role="tablist" aria-label="Partes do app" className="mt-6 mb-6 flex gap-1 overflow-x-auto border-b border-border">
        {tabs.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={current === t.key}
            aria-controls={`aba-${t.key}`}
            onClick={() => go(t.key)}
            className={cn(
              "-mb-px inline-flex shrink-0 items-center gap-1.5 border-b-2 border-transparent px-3 py-2 text-sm whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground",
              current === t.key && "border-foreground font-medium text-foreground",
            )}
          >
            {t.label}
            {!!t.count && (
              <span aria-label={`${t.count} ${t.count === 1 ? "pendência" : "pendências"}`} className="rounded-full border border-warn/35 bg-warn-soft px-1.5 text-[11px] leading-4 text-warn tabular-nums">
                {t.count}
              </span>
            )}
          </button>
        ))}
      </div>

      {scheduling && (
        <ScheduleDialog appId={app.id} appName={m.name} manifest={m} onClose={() => setScheduling(false)}
          onSaved={() => { setScheduling(false); setSchedulesKey((k) => k + 1); toast.success("Agendamento salvo.") }} />
      )}

      {panel("inicio", (
        <div className="flex flex-col">
          {/* Only a stop shows here; approving or suspending a running app lives in Configurações. */}
          {(control?.suspended || waitingApproval) && <AppControlBar className="mb-6" appId={app.id} control={control} onChanged={(c) => setApp({ ...app, control: c })} />}
          {!readable && !cares && (
            <p role="alert" className="mb-4 flex gap-2 text-sm text-warn">
              <TriangleAlert className="mt-0.5 size-4 shrink-0" /> Não consegui ler a pasta do app agora.
            </p>
          )}
          {cares && todos.length > 0 && (
            <section aria-label="Para resolver" className="mb-6">
              <h2 className="mb-2.5 text-sm font-semibold">Para resolver</h2>
              <TodoList todos={todos} />
            </section>
          )}
          {app.shared && !can.restore && <SharedSection app={app} can={can} onChanged={sharedChanged} />}
          {showRuns && (
            <Section title={can.runs ? "Últimas execuções" : "Suas últimas execuções"} aside={(runs?.length ?? 0) > 0 && (
              <button type="button" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline" onClick={() => go("execucoes")}>Ver todas</button>
            )}>
              <RunList runs={runs && runs.slice(0, 3)} everyone={can.runs} />
            </Section>
          )}
          {m.kind === "script" && <SchedulesSection appId={app.id} appName={m.name} manifest={m} reloadKey={schedulesKey} />}
          <Section title="O que ele pede"><InputList inputs={m.inputs ?? []} /></Section>
          <Section title="O que ele entrega"><OutputList outputs={m.outputs} /></Section>
          <Section title="Quem cuida"><People owner={m.owner?.name ?? ""} backup={m.backup?.name ?? ""} /></Section>
        </div>
      ))}

      {showRuns && panel("execucoes", <RunsPanel appId={app.id} everyone={can.runs} reloadKey={signal} />)}

      {panel("documentacao", (
        <div className="[&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
          {can.edit && missingDocs.length > 0 && (
            <p className="mb-4 flex gap-2 text-sm text-warn">
              <TriangleAlert className="mt-0.5 size-4 shrink-0" /> Falta preencher: {missingDocs.join(", ")}.
            </p>
          )}
          {open === "sobre" && draft ? (
            <Section title="Sobre">
              <div className="flex max-w-2xl flex-col gap-3" role="group" aria-label="Sobre">
                <IdentityEditor value={draft.manifest} onChange={(v) => setDraft({ ...draft, manifest: v })} areas={lists.areas} areaLocked={!can.access} />
                <SaveBar dirty={dirty} onSave={save} onCancel={close} extra={<Button size="sm" variant="outline" onClick={reload}>Recarregar</Button>} />
              </div>
            </Section>
          ) : (
            <Section title="Resumo" aside={can.edit && <EditLink part="Resumo" onClick={() => editIn("sobre")} />}>
              <p className={cn("text-sm", !m.summary && "text-muted-foreground")}>{m.summary || "Ainda não escrito."}</p>
            </Section>
          )}
          {part("pede", "O que ele pede", <InputList inputs={m.inputs ?? []} />, () => (
            <InputsEditor value={draft!.manifest.inputs ?? []} onChange={(v) => setManifest({ inputs: v })} />
          ))}
          {part("entrega", "O que ele entrega", <OutputList outputs={m.outputs} />, () => (
            <OutputsEditor value={draft!.manifest.outputs ?? []} onChange={(v) => setManifest({ outputs: v })} />
          ))}
          {part("pessoas", "Quem cuida", <People owner={m.owner?.name ?? ""} backup={m.backup?.name ?? ""} />, () => (
            <PeopleEditor people={lists.people} ownerId={draft!.ownerId} backupId={draft!.backupId} onChange={(o, b) => setDraft({ ...draft!, ownerId: o, backupId: b })} />
          ), can.edit && can.access)}
          {can.tech && (
            <>
              {part("notas", "Pontos de atenção", <Notes notes={notes} />, () => (
                <NotesEditor value={draft!.manifest.notes ?? []} onChange={(v) => setManifest({ notes: v })} />
              ))}
              {showOthers && part("outros", "Outros apps", <OtherApps uses={uses} usedBy={usedBy} />, () => (
                <DependsEditor value={draft!.manifest.depends_on ?? []} onChange={(v) => setManifest({ depends_on: v })} apps={lists.apps} selfSlug={m.id} />
              ))}
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
                <p className="mt-1.5 pl-6 text-xs text-muted-foreground">A documentação também fica salva nessa pasta.</p>
              </Section>
              {showBuild && part("codigo", "Código-fonte", <BuildView build={m.build ?? null} />, () => (
                <BuildEditor value={draft!.manifest.build ?? null} onChange={(v) => setManifest({ build: v })} />
              ))}
              {deps.length > 0 && (
                <Section title="Bibliotecas">
                  <p className="flex flex-wrap gap-1.5">
                    {deps.map((d) => <code key={d} className="rounded-sm bg-secondary px-1.5 py-0.5 font-mono text-xs">{d}</code>)}
                  </p>
                </Section>
              )}
              {(m.secrets ?? []).length > 0 && (
                <Section title="Chaves no código">
                  <p className="flex gap-2 text-sm">
                    <KeyRound className="mt-0.5 size-4 shrink-0 text-destructive" />
                    Usa {m.secrets!.length} chave(s) ou senha(s) escritas no código. Os valores nunca aparecem no Ziv. Guarde cada uma em Configurações.
                  </p>
                </Section>
              )}
            </>
          )}
        </div>
      ))}

      {showSettings && panel("configuracoes", (
        <div className="flex flex-col gap-8">
          {hasSettings && <SettingsSection appId={app.id} kind={m.kind} onChanged={() => sharedChanged()} />}
          {control?.can_manage && !control.suspended && !waitingApproval && (
            <AppControlBar className="" appId={app.id} control={control} onChanged={(c) => setApp({ ...app, control: c })} />
          )}
          {can.tech && person && <AccessSection target={{ kind: "app", id: app.id, can: app.can, areaName: app.manifest.area }} me={person} />}
          {/* Arquivar (ADR 031): only the owner or an administrator. */}
          {can.access && <ArchiveApp appId={app.id} name={m.name} shared={!!app.shared && app.shared.status !== "draft"} />}
        </div>
      ))}

      {serverPart && panel("servidor", (
        <div className="[&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
          {app.shared ? <SharedSection app={app} can={can} onChanged={sharedChanged} /> : <MakeShared app={app} onChanged={setApp} />}
          {app.shared && can.tech && <Tech app={app} onChanged={sharedChanged} />}
          {testLocal && (
            <Section title="Testar antes de publicar">
              <p className="text-sm text-muted-foreground">Roda a cópia da sua pasta no seu computador. A equipe continua usando a versão publicada.</p>
              <Button variant="outline" size="sm" className="mt-3 gap-2" onClick={() => run(target)} disabled={busy.has(app.id)}>
                <Play className="size-4" /> Testar no meu computador
              </Button>
            </Section>
          )}
        </div>
      ))}

      {can.tech && panel("historico", (
        <HistoryTimeline preview={5} appId={app.id} canEdit={can.edit} canRestore={can.restore} reloadKey={historyKey} onChanged={() => void api.app(app.id).then(setApp, () => {})} />
      ))}

      {/* Who only uses the app has no Configurações tab; the owner may still archive. */}
      {!showSettings && can.access && <ArchiveApp className="mt-12" appId={app.id} name={m.name} shared={!!app.shared && app.shared.status !== "draft"} />}
    </article>
  )
}
