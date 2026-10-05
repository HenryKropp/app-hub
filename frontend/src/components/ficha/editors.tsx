import { useState, type ReactNode } from "react"
import { Plus, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Field, NativeSelect } from "@/components/bits"
import type { Input as AppInput, Area, AppSummary, Manifest, Output, PersonRow } from "@/lib/api"
import { INPUT_SOURCES, INPUT_TYPES, KINDS } from "@/lib/kinds"

type Edit<T> = { value: T; onChange: (v: T) => void }

/** Small badge that asks the person to double-check a guessed value. */
export function Confirm({ on }: { on: boolean }) {
  if (!on) return null
  return <span className="ml-1.5 rounded-full bg-warn-soft px-2 py-px align-middle text-[0.7rem] font-medium text-warn">confirmar</span>
}

const cleanLines = (text: string) => text.split("\n").map((s) => s.trim()).filter(Boolean)

/** Lists that the person must still give options to, e.g. ['as opções de "Docs"']. */
export function choiceProblems(inputs: AppInput[]): string[] {
  return inputs
    .filter((i) => i.type === "choice" && !(i.options ?? []).some((o) => o.trim()))
    .map((i) => `as opções de "${i.label.trim() || "entrada"}"`)
}

/** areaLocked: only who cares for the app's access may move it to another area. */
export function IdentityEditor({ value: m, onChange, areas, asks, areaLocked }: Edit<Manifest> & { areas: Area[]; asks?: Set<string>; areaLocked?: boolean }) {
  const set = <K extends keyof Manifest>(k: K, v: Manifest[K]) => onChange({ ...m, [k]: v })
  const known = areas.some((a) => a.name === m.area)
  const ask = asks ?? new Set<string>()
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Field label="Nome" htmlFor="f-name" className="sm:col-span-2">
        <Input id="f-name" value={m.name} onChange={(e) => set("name", e.target.value)} className="h-10 text-base" />
      </Field>
      <Field label={<>O que ele faz<Confirm on={ask.has("summary")} /></>} htmlFor="f-summary" className="sm:col-span-2" hint="Uma ou duas frases, para quem nunca usou.">
        <Textarea id="f-summary" value={m.summary} onChange={(e) => set("summary", e.target.value)} rows={2} />
      </Field>
      <Field label={<>Tipo<Confirm on={ask.has("kind")} /></>} htmlFor="f-kind">
        <NativeSelect id="f-kind" value={m.kind} onChange={(e) => set("kind", e.target.value as Manifest["kind"])}>
          {KINDS.map((k) => (
            <option key={k.value} value={k.value}>
              {k.label}
            </option>
          ))}
        </NativeSelect>
      </Field>
      {areaLocked ? (
        <div className="flex flex-col gap-1.5 text-sm">
          <span className="font-medium">Área</span>
          <p>{m.area}</p>
          <p className="text-xs text-muted-foreground">Só quem cuida do acesso do app troca a área.</p>
        </div>
      ) : (
        <Field label={<>Área<Confirm on={ask.has("area") && !m.area.trim()} /></>} htmlFor="f-area">
          <NativeSelect id="f-area" value={m.area} onChange={(e) => set("area", e.target.value)}>
            <option value="">Escolha a área</option>
            {m.area && !known && <option value={m.area}>{m.area} (fora da lista)</option>}
            {areas.map((a) => (
              <option key={a.id} value={a.name}>
                {a.name}
              </option>
            ))}
          </NativeSelect>
        </Field>
      )}
    </div>
  )
}

export function InputsEditor({ value, onChange }: Edit<AppInput[]>) {
  const setInput = (i: number, patch: Partial<AppInput>) => onChange(value.map((x, j) => (j === i ? { ...x, ...patch } : x)))
  const setType = (i: number, type: AppInput["type"]) =>
    onChange(
      value.map((x, j) => {
        if (j !== i) return x
        if (type === "choice") return { ...x, type }
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        const { options, multiple, ...rest } = x
        return { ...rest, type }
      }),
    )
  const add = () =>
    onChange([...value, { id: `entrada_${value.length + 1}`, label: "", type: "file", source: "fixed_file", path: "", required: true }])

  return (
    <>
      <ul className="flex flex-col gap-3">
        {value.map((inp, i) => {
          const name = inp.label || "entrada"
          return (
            <li key={i} className="grid grid-cols-[1fr_auto] gap-2 rounded-md border border-border bg-background/50 p-3 sm:grid-cols-[1fr_9rem_auto]" data-testid="input-row">
              <Input aria-label="Nome da entrada" value={inp.label} placeholder="Nome da entrada" onChange={(e) => setInput(i, { label: e.target.value })} />
              <NativeSelect aria-label={`Tipo de ${name}`} className="col-span-2 sm:col-span-1" value={inp.type} onChange={(e) => setType(i, e.target.value as AppInput["type"])}>
                {INPUT_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </NativeSelect>
              <Button variant="ghost" size="icon" className="col-start-2 row-start-1 sm:col-start-3" aria-label={`Remover ${name}`} onClick={() => onChange(value.filter((_, j) => j !== i))}>
                <X />
              </Button>
              {inp.type === "choice" && (
                <div className="col-span-full flex flex-col gap-2 pl-1">
                  <Textarea
                    aria-label={`Opções de ${name}`}
                    rows={Math.max(2, (inp.options ?? []).length + 1)}
                    placeholder="Uma opção por linha"
                    value={(inp.options ?? []).join("\n")}
                    onChange={(e) => setInput(i, { options: e.target.value.split("\n") })}
                    onBlur={(e) => setInput(i, { options: cleanLines(e.target.value) })}
                  />
                  <div className="flex items-center gap-2 text-sm">
                    <Checkbox aria-labelledby={`f-multi-${i}-label`} checked={!!inp.multiple} onCheckedChange={(c) => setInput(i, { multiple: c === true })} />
                    <span id={`f-multi-${i}-label`} onClick={() => setInput(i, { multiple: !inp.multiple })}>Pode marcar mais de uma</span>
                  </div>
                </div>
              )}
              <p className="col-span-full text-xs text-muted-foreground">
                Via {INPUT_SOURCES[inp.source]}
                {inp.path && (
                  <>
                    : <code className="font-mono">{inp.path}</code>
                  </>
                )}
              </p>
            </li>
          )
        })}
      </ul>
      {value.length === 0 && <p className="mb-2 text-sm text-muted-foreground">Nenhuma entrada. O app roda sem precisar de nada.</p>}
      <Button variant="outline" size="sm" className="mt-3" onClick={add}>
        <Plus /> Adicionar entrada
      </Button>
    </>
  )
}

export function OutputsEditor({ value, onChange }: Edit<Output[]>) {
  const setOut = (i: number, patch: Partial<Output>) => onChange(value.map((x, j) => (j === i ? { ...x, ...patch } : x)))
  return (
    <>
      <ul className="flex flex-col gap-2">
        {value.map((o, i) => (
          <li key={i} className="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
            <Input aria-label="Descrição do resultado" value={o.label} onChange={(e) => setOut(i, { label: e.target.value })} />
            <Input aria-label="Nome do arquivo" value={o.pattern} className="font-mono text-xs" onChange={(e) => setOut(i, { pattern: e.target.value })} />
            <Button variant="ghost" size="icon" aria-label="Remover resultado" onClick={() => onChange(value.filter((_, j) => j !== i))}>
              <X />
            </Button>
          </li>
        ))}
      </ul>
      <Button variant="outline" size="sm" className="mt-3" onClick={() => onChange([...value, { label: "Arquivo gerado", pattern: "*.xlsx" }])}>
        <Plus /> Adicionar resultado
      </Button>
    </>
  )
}

export function NotesEditor({ value, onChange }: Edit<string[]>) {
  // Raw text is kept locally so a line being typed is not swallowed.
  const [text, setText] = useState(() => value.join("\n"))
  // The parent changed the list from outside (e.g. cancel): follow it. Comparing
  // cleaned lines keeps blank lines being typed from being wiped.
  if (cleanLines(text).join("\n") !== cleanLines(value.join("\n")).join("\n")) setText(value.join("\n"))
  return (
    <Textarea
      aria-label="Observações"
      rows={Math.max(3, text.split("\n").length + 1)}
      placeholder="Uma observação por linha"
      value={text}
      onChange={(e) => {
        setText(e.target.value)
        onChange(cleanLines(e.target.value))
      }}
    />
  )
}

export function DependsEditor({ value, onChange, apps, selfSlug }: Edit<string[]> & { apps: AppSummary[]; selfSlug: string }) {
  const others = apps.filter((a) => a.slug !== selfSlug)
  const unknown = value.filter((id) => id === selfSlug || !apps.some((a) => a.slug === id))
  const toggle = (slug: string, on: boolean) => onChange(on ? [...value, slug] : value.filter((v) => v !== slug))
  return (
    <div className="flex flex-col gap-2">
      {unknown.map((id) => (
        <p key={id} className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>
            {id} · {id === selfSlug ? "é este mesmo app" : "fora do catálogo"}
          </span>
          <Button variant="ghost" size="sm" aria-label={`Tirar ${id}`} onClick={() => onChange(value.filter((v) => v !== id))}>
            Tirar
          </Button>
        </p>
      ))}
      {others.length === 0 && unknown.length === 0 && <p className="text-sm text-muted-foreground">Não há outros apps no catálogo.</p>}
      {others.map((a) => (
        <div key={a.slug} className="flex items-center gap-2 text-sm">
          <Checkbox aria-labelledby={`f-dep-${a.slug}-label`} checked={value.includes(a.slug)} onCheckedChange={(c) => toggle(a.slug, c === true)} />
          <span id={`f-dep-${a.slug}-label`} onClick={() => toggle(a.slug, !value.includes(a.slug))}>{a.name}</span>
        </div>
      ))}
    </div>
  )
}

export function BuildEditor({ value, onChange }: Edit<Manifest["build"]>) {
  const b = value ?? { source: "", command: "" }
  const change = (patch: Partial<NonNullable<Manifest["build"]>>) => {
    const next = { ...b, ...patch }
    onChange(!next.source.trim() && !next.command.trim() ? null : next)
  }
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Field label="Pasta do código-fonte" htmlFor="f-build-source" hint="Onde está o código que gera o programa.">
        <Input id="f-build-source" value={b.source} onChange={(e) => change({ source: e.target.value })} className="font-mono text-xs" />
      </Field>
      <Field label="Comando que gera o programa" htmlFor="f-build-cmd">
        <Input id="f-build-cmd" value={b.command} onChange={(e) => change({ command: e.target.value })} className="font-mono text-xs" />
      </Field>
    </div>
  )
}

export function PeopleEditor({
  people,
  ownerId,
  backupId,
  onChange,
}: {
  people: PersonRow[]
  ownerId: number
  backupId: number
  onChange: (ownerId: number, backupId: number) => void
}): ReactNode {
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Field label="Responsável" htmlFor="f-owner" hint="Quem mantém o app e responde por ele.">
        <NativeSelect id="f-owner" value={ownerId} onChange={(e) => onChange(Number(e.target.value), backupId)}>
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
        <NativeSelect id="f-backup" value={backupId} onChange={(e) => onChange(ownerId, Number(e.target.value))}>
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
  )
}
