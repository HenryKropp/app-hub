import { useEffect, useMemo, useState, type FormEvent } from "react"
import { Link, useNavigate } from "react-router-dom"
import { ArrowLeft } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Field, NativeSelect, PageHeader } from "@/components/bits"
import { ArtifactSourceField, readAsBase64, sourceReady, type ArtifactSource } from "@/components/ArtifactSourceField"
import { ApiError, api, type Area, type ArtifactInput } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { canRegister } from "@/lib/permissions"
import { parseTags } from "@/lib/tags"
import { cn } from "@/lib/utils"

export function NewArtifactPage() {
  const { person } = useAuth()
  const navigate = useNavigate()
  const [name, setName] = useState("")
  const [summary, setSummary] = useState("")
  const [tagsText, setTagsText] = useState("")
  const [areaId, setAreaId] = useState("")
  const [who, setWho] = useState<"all" | "chosen">("all")
  const [source, setSource] = useState<ArtifactSource>({ kind: "link", url: "" })
  const [areas, setAreas] = useState<Area[]>([])
  const [known, setKnown] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [secrets, setSecrets] = useState(0)

  useEffect(() => {
    api.areas().then((a) => {
      setAreas(a)
      setAreaId((cur) => cur || (a[0] ? String(a[0].id) : ""))
    }, (e: Error) => setError(e.message))
    api.artifacts().then((list) => setKnown([...new Set(list.flatMap((a) => a.tags))].sort()), () => {})
  }, [])

  const tags = useMemo(() => parseTags(tagsText), [tagsText])
  const suggestions = known.filter((t) => !tags.includes(t)).slice(0, 12)

  if (!canRegister(person)) {
    return (
      <>
        <PageHeader title="Novo artifact" />
        <p className="mb-4 text-muted-foreground">Você não tem permissão para criar artifacts.</p>
        <Link to="/artifacts" className={cn(buttonVariants({ variant: "outline" }), "h-9 gap-1.5")}>
          <ArrowLeft className="size-4" />
          Voltar aos artifacts
        </Link>
      </>
    )
  }

  const addTag = (t: string) => setTagsText(tags.length ? `${tags.join(", ")}, ${t}` : t)
  const ready = name.trim() !== "" && areaId !== "" && sourceReady(source)

  async function send(confirm: boolean) {
    setBusy(true)
    setError("")
    try {
      const body: ArtifactInput = { name: name.trim(), summary: summary.trim(), tags, area_id: Number(areaId), kind: source.kind }
      if (source.kind === "link") body.url = source.url.trim()
      else {
        body.file_name = source.file!.name
        body.content_base64 = await readAsBase64(source.file!)
      }
      if (confirm) body.confirm_secrets = true
      const made = await api.createArtifact(body)
      navigate(who === "chosen" ? `/artifacts/${made.id}#acesso` : `/artifacts/${made.id}`)
    } catch (e) {
      const data = e instanceof ApiError ? (e.body as { code?: string; secret_count?: number } | undefined) : undefined
      if (e instanceof ApiError && e.status === 409 && data?.code === "artifact_secrets_confirmation_required") {
        setSecrets(data.secret_count ?? 1)
      } else {
        setError((e as Error).message)
      }
      setBusy(false)
    }
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (ready && !busy) void send(false)
  }

  return (
    <>
      <PageHeader title="Novo artifact" />
      <form onSubmit={submit} className="flex max-w-2xl flex-col gap-6">
        <Field label="De onde vem?">
          <ArtifactSourceField value={source} onChange={(v) => { setSource(v); setSecrets(0) }} />
        </Field>
        <Field label="Nome" htmlFor="art-name">
          <Input id="art-name" value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Descrição" htmlFor="art-summary" hint="Opcional. Uma frase sobre o que a página mostra.">
          <Textarea id="art-summary" value={summary} onChange={(e) => setSummary(e.target.value)} rows={2} />
        </Field>
        <Field label="Etiquetas" htmlFor="art-tags" hint="Separe por vírgula.">
          <Input id="art-tags" value={tagsText} onChange={(e) => setTagsText(e.target.value)} placeholder="vendas, mensal" />
          {suggestions.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {suggestions.map((t) => (
                <button key={t} type="button" onClick={() => addTag(t)} className="rounded-full border border-border px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground">
                  {t}
                </button>
              ))}
            </div>
          )}
        </Field>
        <Field label="Área" htmlFor="art-area">
          <NativeSelect id="art-area" value={areaId} onChange={(e) => setAreaId(e.target.value)}>
            {areas.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </NativeSelect>
        </Field>
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-1.5 text-sm font-medium">Quem vê</legend>
          {([["all", "Todos"], ["chosen", "Só quem eu escolher"]] as const).map(([v, label]) => (
            <label key={v} className="flex cursor-pointer items-center gap-2 text-sm">
              <input type="radio" name="who" checked={who === v} onChange={() => setWho(v)} className="accent-[var(--brand)]" />
              {label}
            </label>
          ))}
          {who === "chosen" && <p className="text-xs text-muted-foreground">Depois de criar, você escolhe quem pode abrir.</p>}
        </fieldset>

        {secrets > 0 && (
          <div role="alertdialog" aria-label="Possíveis senhas" className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
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
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex gap-2">
          <Button type="submit" variant="run" disabled={!ready || busy} className="[&_svg]:fill-none">
            {busy ? "Criando…" : "Criar artifact"}
          </Button>
          <Link to="/artifacts" className={buttonVariants({ variant: "outline" })}>
            Cancelar
          </Link>
        </div>
      </form>
    </>
  )
}
