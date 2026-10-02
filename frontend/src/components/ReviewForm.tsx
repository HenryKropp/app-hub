import { useMemo, useState, type ReactNode } from "react"
import { CircleHelp, FileCog, KeyRound, Plus, Search, TriangleAlert, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Field, KindTab, NativeSelect } from "@/components/bits"
import { api, ApiError, type AppDetail, type Input as AppInput, type Manifest, type Person, type ScanItem } from "@/lib/api"
import { AREAS, INPUT_SOURCES, INPUT_TYPES, KINDS, RUNNABLE } from "@/lib/kinds"
import { cn } from "@/lib/utils"

type Props = {
  item: ScanItem
  people: Person[]
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

function Confirm({ on }: { on: boolean }) {
  if (!on) return null
  return <span className="ml-1.5 rounded-full bg-warn-soft px-2 py-px align-middle text-[0.7rem] font-medium text-warn">confirmar</span>
}

function personIdFor(people: Person[], email?: string) {
  return people.find((p) => p.email === email)?.id ?? 0
}

/** Missing items that block publishing, in plain Portuguese. */
export function missingForPublish(m: Manifest, ownerId: number, backupId: number): string[] {
  const out: string[] = []
  if (!m.name.trim()) out.push("o nome")
  if (!ownerId) out.push("o responsável")
  if (ownerId && backupId && ownerId === backupId) out.push("um reserva diferente do responsável")
  if (RUNNABLE.includes(m.kind) && !m.entrypoint.trim()) out.push("o arquivo que inicia o app")
  if ((m.inputs ?? []).some((i) => !i.label.trim())) out.push("o nome de cada entrada")
  return out
}

export function ReviewForm({ item, people, me, onPublished, onSkip }: Props) {
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
  const setInput = (i: number, patch: Partial<AppInput>) => set("inputs", (m.inputs ?? []).map((x, j) => (j === i ? { ...x, ...patch } : x)))

  const runnable = RUNNABLE.includes(m.kind)
  const missing = missingForPublish(m, ownerId, backupId)
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

  const addInput = () => {
    const n = (m.inputs ?? []).length + 1
    set("inputs", [...(m.inputs ?? []), { id: `entrada_${n}`, label: "", type: "file", source: "fixed_file", path: "", required: true }])
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
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Nome" htmlFor="f-name" className="sm:col-span-2">
            <Input id="f-name" value={m.name} onChange={(e) => set("name", e.target.value)} className="h-10 text-base" />
          </Field>
          <Field label={<>O que ele faz<Confirm on={asks.has("summary")} /></>} htmlFor="f-summary" className="sm:col-span-2" hint="Uma ou duas frases, para quem nunca usou.">
            <Textarea id="f-summary" value={m.summary} onChange={(e) => set("summary", e.target.value)} rows={2} />
          </Field>
          <Field label={<>Tipo<Confirm on={asks.has("kind")} /></>} htmlFor="f-kind">
            <NativeSelect id="f-kind" value={m.kind} onChange={(e) => set("kind", e.target.value as Manifest["kind"])}>
              {KINDS.map((k) => (
                <option key={k.value} value={k.value}>
                  {k.label}
                </option>
              ))}
            </NativeSelect>
          </Field>
          <Field label={<>Área<Confirm on={asks.has("area") && !m.area.trim()} /></>} htmlFor="f-area">
            <Input id="f-area" list="areas" value={m.area} onChange={(e) => set("area", e.target.value)} placeholder="Ex.: Investimentos" />
            <datalist id="areas">
              {AREAS.map((a) => (
                <option key={a} value={a} />
              ))}
            </datalist>
          </Field>
        </div>
      </Block>

      <Block title="Pessoas">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Responsável" htmlFor="f-owner" hint="Quem mantém o app e responde por ele.">
            <NativeSelect id="f-owner" value={ownerId} onChange={(e) => setOwnerId(Number(e.target.value))}>
              <option value={0}>Escolha…</option>
              {people.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </NativeSelect>
          </Field>
          <Field
            label="Reserva"
            htmlFor="f-backup"
            hint={backupId ? "Quem assume se o responsável não estiver." : <span className="text-warn">Sem reserva, o app fica marcado como risco.</span>}
          >
            <NativeSelect id="f-backup" value={backupId} onChange={(e) => setBackupId(Number(e.target.value))}>
              <option value={0}>Ninguém por enquanto</option>
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
          <ul className="flex flex-col gap-3">
            {(m.inputs ?? []).map((inp, i) => (
              <li key={i} className="grid gap-2 rounded-md border border-border bg-background/50 p-3 sm:grid-cols-[1fr_9rem_auto]" data-testid="input-row">
                <Input aria-label="Nome da entrada" value={inp.label} placeholder="Nome da entrada" onChange={(e) => setInput(i, { label: e.target.value })} />
                <NativeSelect aria-label="Tipo da entrada" value={inp.type} onChange={(e) => setInput(i, { type: e.target.value as AppInput["type"] })}>
                  {INPUT_TYPES.map((t) => (
                    <option key={t.value} value={t.value}>
                      {t.label}
                    </option>
                  ))}
                </NativeSelect>
                <Button variant="ghost" size="icon" aria-label={`Remover ${inp.label || "entrada"}`} onClick={() => set("inputs", (m.inputs ?? []).filter((_, j) => j !== i))}>
                  <X />
                </Button>
                <p className="text-xs text-muted-foreground sm:col-span-3">
                  Via {INPUT_SOURCES[inp.source]}
                  {inp.path && (
                    <>
                      : <code className="font-mono">{inp.path}</code>
                    </>
                  )}
                </p>
              </li>
            ))}
          </ul>
          {(m.inputs ?? []).length === 0 && <p className="mb-2 text-sm text-muted-foreground">Nenhuma entrada. O app roda sem precisar de nada.</p>}
          <Button variant="outline" size="sm" className="mt-3" onClick={addInput}>
            <Plus /> Adicionar entrada
          </Button>
        </Block>
      )}

      {runnable && (
        <Block title="O que ele devolve">
          <ul className="flex flex-col gap-2">
            {(m.outputs ?? []).map((o, i) => (
              <li key={i} className="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
                <Input aria-label="Descrição do resultado" value={o.label} onChange={(e) => set("outputs", (m.outputs ?? []).map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
                <Input aria-label="Nome do arquivo" value={o.pattern} className="font-mono text-xs" onChange={(e) => set("outputs", (m.outputs ?? []).map((x, j) => (j === i ? { ...x, pattern: e.target.value } : x)))} />
                <Button variant="ghost" size="icon" aria-label="Remover resultado" onClick={() => set("outputs", (m.outputs ?? []).filter((_, j) => j !== i))}>
                  <X />
                </Button>
              </li>
            ))}
          </ul>
          <Button variant="outline" size="sm" className="mt-3" onClick={() => set("outputs", [...(m.outputs ?? []), { label: "Arquivo gerado", pattern: "*.xlsx" }])}>
            <Plus /> Adicionar resultado
          </Button>
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
