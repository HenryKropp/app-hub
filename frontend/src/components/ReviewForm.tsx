import { useMemo, useState, type ReactNode } from "react"
import { CircleHelp, FileCog, KeyRound, Search, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, KindTab, NativeSelect } from "@/components/bits"
import { BuildEditor, choiceProblems, Confirm, DependsEditor, IdentityEditor, InputsEditor, OutputsEditor, PeopleEditor } from "@/components/ficha/editors"
import { api, ApiError, type AppDetail, type Area, type AppSummary, type Manifest, type Person, type PersonRow, type ScanItem } from "@/lib/api"
import { RUNNABLE } from "@/lib/kinds"
import { cn } from "@/lib/utils"

type Props = {
  item: ScanItem
  people: PersonRow[]
  areas: Area[]
  apps: AppSummary[]
  me: Person
  onPublished: (app: AppDetail) => void
  onSkip: () => void
}

function Block({ title, icon, children, id }: { title: string; icon?: ReactNode; children: ReactNode; id?: string }) {
  return (
    <section id={id} className="border-t border-border py-6 first:border-t-0 first:pt-0">
      <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold">
        {icon}
        {title}
      </h3>
      {children}
    </section>
  )
}

function personIdFor(people: PersonRow[], email?: string) {
  return people.find((p) => p.email === email)?.id ?? 0
}

/** Missing items that block publishing, in plain Portuguese. */
export function missingForPublish(m: Manifest, ownerId: number, backupId: number, areas: Area[] = []): string[] {
  const out: string[] = []
  if (!m.name.trim()) out.push("o nome")
  if (!ownerId) out.push("o responsável")
  if (ownerId && backupId && ownerId === backupId) out.push("um reserva diferente do responsável")
  if (RUNNABLE.includes(m.kind) && !m.entrypoint.trim()) out.push("o arquivo que inicia o app")
  if ((m.inputs ?? []).some((i) => !i.label.trim())) out.push("o nome de cada entrada")
  out.push(...choiceProblems(m.inputs ?? []))
  if (!m.area || !areas.some((a) => a.name === m.area)) out.push("a área")
  return out
}

export function ReviewForm({ item, people, areas, apps, me, onPublished, onSkip }: Props) {
  const [m, setM] = useState<Manifest>(() => ({
    ...item.manifest,
    inputs: item.manifest.inputs ?? [],
    outputs: item.manifest.outputs ?? [],
  }))
  const [ownerId, setOwnerId] = useState<number>(() => personIdFor(people, item.manifest.owner?.email) || me.id)
  const [backupId, setBackupId] = useState<number>(() => personIdFor(people, item.manifest.backup?.email))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const asks = useMemo(() => new Set((item.questions ?? []).map((q) => q.field)), [item.questions])
  const set = <K extends keyof Manifest>(k: K, v: Manifest[K]) => setM((cur) => ({ ...cur, [k]: v }))

  const runnable = RUNNABLE.includes(m.kind)
  const missing = missingForPublish(m, ownerId, backupId, areas)
  const entryOptions = Array.from(new Set([...(item.entrypoint_options ?? []), m.entrypoint].filter(Boolean)))
  const updating = item.registered_app_id > 0 || item.has_manifest

  const publish = async () => {
    setBusy(true)
    setError("")
    try {
      const saved = await api.saveApp({ folder_path: item.folder_path, owner_id: ownerId, backup_id: backupId || null, manifest: m })
      onPublished(saved)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Não foi possível publicar.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col" data-testid="review-form">
      <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <KindTab kind={m.kind} />
          <h2 className="mt-2 font-heading text-3xl leading-tight">{m.name || item.folder_name}</h2>
          <p className="mt-1 font-mono text-xs break-all text-muted-foreground">{item.folder_display}</p>
        </div>
      </div>

      {(item.questions ?? []).length > 0 && (
        <div className="mb-6 rounded-lg border border-warn/40 bg-warn-soft/60 p-4">
          <p className="flex items-center gap-2 text-sm font-semibold">
            <CircleHelp className="size-4 text-warn" />
            Confirme {item.questions!.length === 1 ? "este ponto" : `estes ${item.questions!.length} pontos`}
          </p>
          <ul className="mt-2 flex list-disc flex-col gap-1 pl-9 text-sm">
            {item.questions!.map((q, i) => (
              <li key={i}>{q.text}</li>
            ))}
          </ul>
        </div>
      )}

      {item.similar.length > 0 && (
        <p className="mb-6 flex gap-2 rounded-lg border border-border bg-muted/60 p-4 text-sm">
          <Search className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <span>
            Parece com <strong>{item.similar.map((s) => s.name).join(", ")}</strong>. Confira se não é o mesmo app antes de publicar.
          </span>
        </p>
      )}

      <Block title="Identidade">
        <IdentityEditor value={m} onChange={setM} areas={areas} asks={asks} />
      </Block>

      <Block title="Pessoas">
        <PeopleEditor
          people={people}
          ownerId={ownerId}
          backupId={backupId}
          onChange={(o, b) => {
            setOwnerId(o)
            setBackupId(b)
          }}
        />
      </Block>

      {runnable && (
        <Block title="Como ele roda" icon={<FileCog className="size-3.5" />}>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={<>Arquivo que inicia o app<Confirm on={asks.has("entrypoint")} /></>} htmlFor="f-entry">
              {entryOptions.length > 1 ? (
                <NativeSelect id="f-entry" value={m.entrypoint} onChange={(e) => set("entrypoint", e.target.value)} className="font-mono text-xs">
                  {entryOptions.map((o) => (
                    <option key={o} value={o}>
                      {o}
                    </option>
                  ))}
                </NativeSelect>
              ) : (
                <Input id="f-entry" value={m.entrypoint} onChange={(e) => set("entrypoint", e.target.value)} className="font-mono text-xs" />
              )}
            </Field>
            <Field label="Comando" htmlFor="f-cmd" hint="Como o Hub vai iniciar o app.">
              <Input id="f-cmd" value={m.run.command} onChange={(e) => set("run", { ...m.run, command: e.target.value })} className="font-mono text-xs" />
            </Field>
            {m.kind === "web" && (
              <Field label="Endereço da tela" htmlFor="f-url">
                <Input id="f-url" value={m.run.url} onChange={(e) => set("run", { ...m.run, url: e.target.value })} className="font-mono text-xs" />
              </Field>
            )}
            {m.kind === "scheduled" && (
              <Field label="Quando roda" htmlFor="f-sched">
                <Input id="f-sched" value={m.schedule} onChange={(e) => set("schedule", e.target.value)} />
              </Field>
            )}
          </div>
          {(m.runtime.dependencies ?? []).length > 0 && (
            <div className="mt-4">
              <p className="mb-2 text-[0.8rem] font-semibold text-foreground/85">
                Bibliotecas {m.runtime.dependencies_file ? `(de ${m.runtime.dependencies_file})` : "(deduzidas do código)"}
              </p>
              <div className="flex flex-wrap gap-1.5">
                {m.runtime.dependencies!.map((d) => (
                  <span key={d} className="rounded-sm bg-secondary px-1.5 py-0.5 font-mono text-[0.7rem]">
                    {d}
                  </span>
                ))}
              </div>
            </div>
          )}
        </Block>
      )}

      {runnable && (
        <Block title="O que ele recebe">
          {asks.has("inputs") && <p className="-mt-2 mb-3 text-sm text-muted-foreground">Remova o que não for entrada.<Confirm on /></p>}
          <InputsEditor value={m.inputs ?? []} onChange={(v) => set("inputs", v)} />
        </Block>
      )}

      {runnable && (
        <Block title="O que ele devolve">
          <OutputsEditor value={m.outputs ?? []} onChange={(v) => set("outputs", v)} />
        </Block>
      )}

      {(m.kind === "executable" || (m.depends_on ?? []).length > 0) && (
        <Block title="Outros apps que ele usa">
          <DependsEditor value={m.depends_on ?? []} onChange={(v) => set("depends_on", v)} apps={apps} selfSlug={m.id} />
        </Block>
      )}

      {(m.kind === "executable" || m.build) && (
        <Block title="Código-fonte">
          <BuildEditor value={m.build ?? null} onChange={(v) => set("build", v)} />
        </Block>
      )}

      {(item.secrets ?? []).length > 0 && (
        <Block title="Chaves e senhas no código" icon={<KeyRound className="size-3.5 text-destructive" />}>
          <div className="rounded-md border border-destructive/30 bg-destructive/5 p-4 text-sm">
            <p>Encontrei chaves ou senhas escritas nestes lugares. O valor não é mostrado nem copiado para o Hub.</p>
            <ul className="mt-3 flex flex-col gap-1 font-mono text-xs">
              {item.secrets!.map((s, i) => (
                <li key={i}>
                  {s.file}:{s.line} · {s.hint}
                </li>
              ))}
            </ul>
            <p className="mt-3 text-muted-foreground">Quando o cofre estiver pronto, o Hub vai oferecer mover cada uma para lá.</p>
          </div>
        </Block>
      )}

      {((item.findings ?? []).length > 0 || (m.notes ?? []).length > 0) && (
        <Block title="O que o Hub encontrou">
          <ul className="flex flex-col gap-1.5 text-sm">
            {(item.findings ?? []).map((f, i) => (
              <li key={`f${i}`} className="flex gap-2">
                <span className="mt-2 size-1 shrink-0 rounded-full bg-muted-foreground" />
                {f}
              </li>
            ))}
            {(m.notes ?? []).map((n, i) => (
              <li key={`n${i}`} className="flex gap-2">
                <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warn" />
                {n}
              </li>
            ))}
          </ul>
        </Block>
      )}

      <div className="sticky bottom-0 -mx-6 mt-4 border-t border-border bg-card px-6 py-4">
        {error && (
          <p role="alert" className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className={cn("max-w-md text-xs", missing.length ? "text-warn" : "text-muted-foreground")}>
            {missing.length
              ? `Falta definir ${missing.join(", ")}.`
              : "Vou criar app-hub.yaml e FICHA.md dentro da pasta do app. Nenhum arquivo do app é alterado."}
          </p>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={onSkip}>
              Pular
            </Button>
            <Button size="lg" className="h-10 px-4" disabled={busy || missing.length > 0} onClick={publish}>
              {busy ? "Publicando…" : updating ? "Atualizar ficha" : "Publicar no catálogo"}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
