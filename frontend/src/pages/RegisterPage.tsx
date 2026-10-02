import { useEffect, useMemo, useState, type FormEvent } from "react"
import { Link } from "react-router-dom"
import { toast } from "sonner"
import { Archive, Check, CircleHelp, Copy, FolderSearch, KeyRound } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { KindTab, PageHeader } from "@/components/bits"
import { ReviewForm } from "@/components/ReviewForm"
import { api, type Person, type ScanItem, type ScanResult } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"

type Status = "pending" | "published" | "skipped"
const PATH_KEY = "apphub-last-scan-path"

function loadPath() {
  try {
    return localStorage.getItem(PATH_KEY) ?? ""
  } catch {
    return ""
  }
}

export function RegisterPage() {
  const { person: me } = useAuth()
  const [path, setPath] = useState(loadPath)
  const [mode, setMode] = useState<"batch" | "single">("batch")
  const [scanning, setScanning] = useState(false)
  const [error, setError] = useState("")
  const [result, setResult] = useState<ScanResult | null>(null)
  const [status, setStatus] = useState<Record<string, Status>>({})
  const [published, setPublished] = useState<Record<string, number>>({})
  const [selected, setSelected] = useState(0)
  const [people, setPeople] = useState<Person[]>([])

  useEffect(() => {
    api.people().then(setPeople, () => {})
  }, [])

  const scan = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setScanning(true)
    try {
      const r = await api.scan(path, mode)
      try {
        localStorage.setItem(PATH_KEY, path)
      } catch {
        /* storage unavailable */
      }
      const initial: Record<string, Status> = {}
      r.items.forEach((it) => (initial[it.folder_path] = it.registered_app_id ? "published" : it.suggest_skip ? "skipped" : "pending"))
      setStatus(initial)
      setPublished({})
      setResult(r)
      const firstPending = r.items.findIndex((it) => initial[it.folder_path] === "pending")
      setSelected(Math.max(0, firstPending))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não consegui analisar a pasta.")
    } finally {
      setScanning(false)
    }
  }

  const items = useMemo(() => result?.items ?? [], [result])
  const counts = useMemo(() => {
    const c = { pending: 0, published: 0, skipped: 0 }
    items.forEach((it) => c[status[it.folder_path] ?? "pending"]++)
    return c
  }, [items, status])

  const advance = (from: number, next: Record<string, Status>) => {
    for (let k = 1; k <= items.length; k++) {
      const j = (from + k) % items.length
      if (next[items[j].folder_path] === "pending") return setSelected(j)
    }
  }

  const mark = (it: ScanItem, s: Status) => {
    const next = { ...status, [it.folder_path]: s }
    setStatus(next)
    advance(selected, next)
  }

  const current = items[selected]

  return (
    <>
      <PageHeader title="Traga os apps para o catálogo" />

      <form onSubmit={scan} className="mb-10 rounded-xl border border-border bg-card p-5 md:p-6">
        <label htmlFor="scan-path" className="text-sm font-semibold">
          Onde estão os apps?
        </label>
        <p className="mt-1 text-sm text-muted-foreground">
          Cole o caminho da pasta, como aparece no Explorador de Arquivos. O Hub só lê: nada é alterado até você publicar.
        </p>
        <div className="mt-4 flex flex-col gap-3 lg:flex-row">
          <Input
            id="scan-path"
            value={path}
            onChange={(e) => setPath(e.target.value)}
            placeholder="C:\Users\voce\Documentos\Apps"
            className="h-10 flex-1 bg-background font-mono text-xs"
            required
          />
          <div className="flex rounded-md border border-border bg-background p-0.5 text-sm" role="radiogroup" aria-label="Tipo de pasta">
            {(
              [
                ["batch", "Pasta com vários apps"],
                ["single", "Um app só"],
              ] as const
            ).map(([v, label]) => (
              <button
                key={v}
                type="button"
                role="radio"
                aria-checked={mode === v}
                onClick={() => setMode(v)}
                className={cn("rounded px-3 py-1.5 transition-colors", mode === v ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground")}
              >
                {label}
              </button>
            ))}
          </div>
          <Button type="submit" size="lg" className="h-10 px-5" disabled={scanning || !path.trim()}>
            <FolderSearch /> {scanning ? "Analisando…" : "Analisar"}
          </Button>
        </div>
        {error && (
          <p role="alert" className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}
      </form>

      {result && items.length === 0 && <p className="text-muted-foreground">Não encontrei apps nessa pasta.</p>}

      {result && items.length > 0 && me && (
        <>
          <div className="mb-5 flex flex-wrap items-baseline gap-x-6 gap-y-2">
            <p className="font-heading text-xl">
              {items.length} {items.length === 1 ? "app encontrado" : "apps encontrados"}
            </p>
            <p className="font-mono text-xs text-muted-foreground">
              {counts.published} no catálogo · {counts.pending} para revisar · {counts.skipped} pulados
            </p>
            {counts.pending === 0 && (
              <Link to="/" className="text-sm font-medium underline underline-offset-4">
                Ver o catálogo →
              </Link>
            )}
          </div>

          <div className="grid gap-6 lg:grid-cols-[19rem_1fr]">
            <ol className="flex flex-col gap-1 lg:sticky lg:top-6 lg:max-h-[calc(100svh-3rem)] lg:overflow-y-auto lg:pr-1" aria-label="Apps encontrados">
              {items.map((it, i) => {
                const s = status[it.folder_path] ?? "pending"
                return (
                  <li key={it.folder_path}>
                    <button
                      type="button"
                      onClick={() => setSelected(i)}
                      aria-current={i === selected}
                      className={cn(
                        "relative w-full rounded-md border px-3 py-2.5 text-left transition-colors",
                        i === selected ? "border-foreground/25 bg-card" : "border-transparent hover:bg-card/60",
                        s === "skipped" && "opacity-55",
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <KindTab kind={it.manifest.kind} className="scale-90 origin-left" />
                        <span className="flex items-center gap-1.5 text-muted-foreground">
                          {(it.questions ?? []).length > 0 && s === "pending" && (
                            <span title="Pontos para confirmar" className="flex items-center gap-0.5 text-[0.7rem] text-warn">
                              <CircleHelp className="size-3.5" />
                              {it.questions!.length}
                            </span>
                          )}
                          {(it.secrets ?? []).length > 0 && <KeyRound className="size-3.5 text-destructive" aria-label="Tem chaves no código" />}
                          {it.similar.length > 0 && <Copy className="size-3.5" aria-label="Parecido com outro app" />}
                          {it.suggest_skip && <Archive className="size-3.5" aria-label="Parece material antigo" />}
                        </span>
                      </div>
                      <p className="mt-1 truncate font-medium">{it.manifest.name}</p>
                      <p className="truncate font-mono text-[0.68rem] text-muted-foreground">{it.folder_name}</p>
                      {s === "published" && (
                        <span className="absolute top-2 right-2 inline-flex items-center gap-1 rounded-full bg-ok-soft px-2 py-0.5 text-[0.7rem] font-medium text-ok">
                          <Check className="size-3" /> no catálogo
                        </span>
                      )}
                    </button>
                  </li>
                )
              })}
            </ol>

            <div className="min-w-0 rounded-xl border border-border bg-card px-6 pt-6">
              {current && status[current.folder_path] === "published" && (
                <p className="mb-5 rounded-md bg-ok-soft px-4 py-3 text-sm">
                  Este app já está no catálogo.{" "}
                  {(published[current.folder_path] || current.registered_app_id) > 0 && (
                    <Link to={`/apps/${published[current.folder_path] || current.registered_app_id}`} className="font-medium underline underline-offset-4">
                      Abrir ficha
                    </Link>
                  )}{" "}
                  Você pode revisar e atualizar abaixo.
                </p>
              )}
              {current && (
                <ReviewForm
                  key={current.folder_path}
                  item={current}
                  people={people}
                  me={me}
                  onPublished={(app) => {
                    setPublished((p) => ({ ...p, [current.folder_path]: app.id }))
                    setResult((r) =>
                      r && {
                        ...r,
                        items: r.items.map((it) => (it.folder_path === current.folder_path ? { ...it, manifest: app.manifest, registered_app_id: app.id, questions: [] } : it)),
                      },
                    )
                    toast.success(`${app.manifest.name} está no catálogo`, { description: "Criei app-hub.yaml e FICHA.md na pasta do app." })
                    mark(current, "published")
                  }}
                  onSkip={() => mark(current, "skipped")}
                />
              )}
            </div>
          </div>
        </>
      )}
    </>
  )
}
