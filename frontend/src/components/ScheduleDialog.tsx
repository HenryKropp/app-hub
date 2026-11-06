import { useEffect, useMemo, useState } from "react"
import { CalendarClock, FolderOpen, LoaderCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field, NativeSelect } from "@/components/bits"
import { MAX_FILE_BYTES, ScriptField, readBase64 } from "@/components/ScriptFields"
import { api, type Computer, type Frequency, type Manifest, type Schedule } from "@/lib/api"
import { DAY_NAMES, formatRunTime, frequencyProblem, upcoming } from "@/lib/schedules"

type Kind = Frequency["kind"]

/** Only the keys the chosen kind uses go to the server. */
function frequencyOf(kind: Kind, every: string, at: string, days: number[], day: string): Frequency {
  switch (kind) {
    case "minutes":
    case "hours": return { kind, every: Number(every) }
    case "daily": return { kind, at }
    case "weekdays": return { kind, at, days: [...days].sort((a, b) => a - b) }
    case "monthly": return { kind, at, day: Number(day) }
  }
}

/**
 * ScheduleDialog makes or edits a schedule of a script (ADR 024): the same
 * fields Rodar asks for, plus when to run and on which computer.
 */
export function ScheduleDialog({ appId, appName, manifest, schedule, onClose, onSaved }: {
  appId: number
  appName: string
  manifest: Manifest
  /** The schedule being edited; absent to make a new one. */
  schedule?: Schedule
  onClose: () => void
  onSaved: (s: Schedule) => void
}) {
  const f = schedule?.frequency
  const [kind, setKind] = useState<Kind>(f?.kind ?? "daily")
  const [every, setEvery] = useState(String(f?.every ?? (f?.kind === "hours" ? 1 : 15)))
  const [at, setAt] = useState(f?.at ?? "07:00")
  const [days, setDays] = useState<number[]>(f?.days ?? [1, 2, 3, 4, 5])
  const [day, setDay] = useState(String(f?.day ?? 1))
  const [name, setName] = useState(schedule?.name ?? "")
  const [folder, setFolder] = useState(schedule?.output_folder ?? "")
  const [values, setValues] = useState<Record<string, string | string[]>>(() =>
    Object.fromEntries(Object.entries(schedule?.inputs ?? {}).map(([id, v]) => {
      const field = manifest.inputs?.find((i) => i.id === id)
      return [id, field?.type === "choice" && field.multiple ? v : (v[0] ?? "")]
    })))
  const [files, setFiles] = useState<Record<string, File>>({})
  // Files saved with the schedule being edited: kept unless replaced or removed.
  const [kept, setKept] = useState<Record<string, string>>(() =>
    Object.fromEntries(Object.entries(schedule?.files ?? {}).map(([id, file]) => [id, file.name])))
  const [computers, setComputers] = useState<Computer[] | null>(null)
  const [computerId, setComputerId] = useState(schedule?.computer_id ?? 0)
  const [error, setError] = useState("")
  const [saving, setSaving] = useState(false)
  const fields = useMemo(() => (manifest.inputs ?? []).filter((i) => i.source !== "window"), [manifest])

  useEffect(() => {
    let live = true
    api.computers().then((info) => {
      if (!live) return
      setComputers(info.computers)
      setComputerId((cur) => cur || (info.computers.find((c) => c.connected) ?? info.computers[0])?.id || 0)
    }, (e: Error) => live && setError(e.message))
    return () => { live = false }
  }, [])

  const frequency = frequencyOf(kind, every, at, days, day)
  const problem = frequencyProblem(frequency)
  const next = upcoming(frequency, new Date(), 3)

  const save = async () => {
    setError("")
    const missing = fields.find((field) => {
      if (!field.required) return false
      if (field.type === "file") return !files[field.id] && !kept[field.id]
      const v = values[field.id]
      return Array.isArray(v) ? v.length === 0 : !String(v ?? "").trim()
    })
    if (missing) {
      setError(`Preencha “${missing.label}” para continuar.`)
      return
    }
    setSaving(true)
    try {
      if (Object.values(files).reduce((sum, file) => sum + file.size, 0) > MAX_FILE_BYTES) {
        throw new Error("Os arquivos selecionados passam de 10 MB no total. Remova algum arquivo e tente novamente.")
      }
      const encoded: Record<string, { name: string; content_base64: string }> = {}
      // An empty content keeps the file already saved with this name.
      for (const [id, fileName] of Object.entries(kept)) encoded[id] = { name: fileName, content_base64: "" }
      await Promise.all(Object.entries(files).map(async ([id, file]) => {
        encoded[id] = { name: file.name, content_base64: await readBase64(file) }
      }))
      const inputs = Object.fromEntries(Object.entries(values).filter(([, v]) => (Array.isArray(v) ? v.length > 0 : v !== "")))
      const form = { name: name.trim(), frequency, computer_id: computerId, output_folder: folder.trim(), inputs, files: encoded }
      const saved = schedule ? await api.updateSchedule(schedule.id, form) : await api.createSchedule(appId, form)
      onSaved(saved)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não consegui salvar o agendamento.")
      setSaving(false)
    }
  }

  const chooseOutput = async () => {
    try {
      const r = await api.chooseFolder(folder)
      if (!r.canceled) setFolder(r.folder)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível abrir a janela de pastas.")
    }
  }

  const chosen = computers?.find((c) => c.id === computerId)
  return (
    <Dialog open onOpenChange={(v) => !v && !saving && onClose()}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{schedule ? "Editar agendamento" : `Agendar ${appName}`}</DialogTitle>
          <DialogDescription>O Ziv roda o script sozinho nos horários escolhidos, no seu computador, com o Agente ligado. Toda execução aparece no histórico.</DialogDescription>
        </DialogHeader>

        {fields.length > 0 && (
          <div className="flex flex-col gap-4 border-y border-border py-4">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Dados para este app</p>
            {fields.map((field) => (
              <div key={field.id}>
                <ScriptField field={field} value={values[field.id]} disabled={saving} onError={setError}
                  onChange={(v) => setValues((cur) => ({ ...cur, [field.id]: v }))}
                  file={files[field.id]}
                  onFile={(file) => {
                    setFiles((cur) => {
                      const nextFiles = { ...cur }
                      if (file) nextFiles[field.id] = file
                      else delete nextFiles[field.id]
                      return nextFiles
                    })
                    setKept((cur) => {
                      const nextKept = { ...cur }
                      delete nextKept[field.id]
                      return nextKept
                    })
                  }} />
                {kept[field.id] && !files[field.id] && <p className="mt-1 text-xs text-muted-foreground">Arquivo guardado: {kept[field.id]}</p>}
              </div>
            ))}
          </div>
        )}

        <Field label="Onde salvar os resultados?" htmlFor="schedule-folder" hint="Uma pasta do computador escolhido abaixo.">
          <div className="flex gap-2">
            <Input id="schedule-folder" value={folder} onChange={(e) => setFolder(e.target.value)} placeholder="Escolha uma pasta" disabled={saving} />
            <Button type="button" variant="outline" className="shrink-0 gap-2" onClick={() => void chooseOutput()} disabled={saving}>
              <FolderOpen className="size-4" /> Escolher…
            </Button>
          </div>
        </Field>

        <fieldset className="flex min-w-0 flex-col gap-3">
          <legend className="mb-1.5 text-sm font-medium">Quando rodar</legend>
          <div className="flex flex-wrap items-end gap-3">
            <Field label="Frequência" htmlFor="schedule-kind" className="min-w-44 flex-1">
              <NativeSelect id="schedule-kind" className="h-10" value={kind} disabled={saving} onChange={(e) => {
                const k = e.target.value as Kind
                setKind(k)
                if (k === "minutes") setEvery("15")
                if (k === "hours") setEvery("1")
              }}>
                <option value="minutes">A cada … minutos</option>
                <option value="hours">A cada … horas</option>
                <option value="daily">Todo dia</option>
                <option value="weekdays">Em dias da semana</option>
                <option value="monthly">Todo mês</option>
              </NativeSelect>
            </Field>
            {(kind === "minutes" || kind === "hours") && (
              <Field label={kind === "minutes" ? "A cada quantos minutos" : "A cada quantas horas"} htmlFor="schedule-every" className="w-44">
                <Input id="schedule-every" type="number" inputMode="numeric" min={kind === "minutes" ? 5 : 1} max={kind === "minutes" ? 1440 : 24} value={every} onChange={(e) => setEvery(e.target.value)} disabled={saving} />
              </Field>
            )}
            {kind === "monthly" && (
              <Field label="No dia" htmlFor="schedule-day" className="w-28">
                <NativeSelect id="schedule-day" className="h-10" value={day} onChange={(e) => setDay(e.target.value)} disabled={saving}>
                  {Array.from({ length: 28 }, (_, i) => <option key={i + 1} value={i + 1}>{i + 1}</option>)}
                </NativeSelect>
              </Field>
            )}
            {kind !== "minutes" && kind !== "hours" && (
              <Field label="Às" htmlFor="schedule-at" className="w-32">
                <Input id="schedule-at" type="time" value={at} onChange={(e) => setAt(e.target.value)} disabled={saving} />
              </Field>
            )}
          </div>
          {kind === "weekdays" && (
            <div className="flex flex-wrap gap-2" role="group" aria-label="Dias da semana">
              {DAY_NAMES.map((label, d) => (
                <label key={d} className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm">
                  <input type="checkbox" checked={days.includes(d)} disabled={saving} className="size-4 accent-[var(--foreground)]"
                    onChange={(e) => setDays((cur) => (e.target.checked ? [...cur, d] : cur.filter((x) => x !== d)))} />
                  {label}
                </label>
              ))}
            </div>
          )}
          {problem
            ? <p className="text-sm text-destructive">{problem}</p>
            : <p className="flex items-start gap-2 text-sm text-muted-foreground"><CalendarClock className="mt-0.5 size-4 shrink-0" /><span>Próximas: {next.map((d) => formatRunTime(d)).join(" · ")}</span></p>}
        </fieldset>

        <Field label="Em qual computador" htmlFor="schedule-computer" hint={chosen && !chosen.connected ? "Este computador está desligado agora. Ele precisa estar ligado, com o Agente aberto, na hora de rodar." : "Ele precisa estar ligado, com o Agente aberto, na hora de rodar."}>
          {computers && computers.length === 0
            ? <p className="text-sm text-muted-foreground">Você ainda não conectou um computador. Conecte em Configurações › Meu computador.</p>
            : <NativeSelect id="schedule-computer" className="h-10" value={computerId || ""} onChange={(e) => setComputerId(Number(e.target.value))} disabled={saving || !computers}>
                {(computers ?? []).map((c) => <option key={c.id} value={c.id}>{c.name}{c.connected ? "" : " (desligado)"}</option>)}
              </NativeSelect>}
        </Field>

        <Field label="Nome (opcional)" htmlFor="schedule-name" hint="Ajuda a achar o agendamento quando o app tem mais de um.">
          <Input id="schedule-name" value={name} maxLength={80} onChange={(e) => setName(e.target.value)} placeholder="Ex.: Relatório da manhã" disabled={saving} />
        </Field>

        {error && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={saving}>Cancelar</Button>
          <Button onClick={() => void save()} disabled={saving || !!problem || !folder.trim() || !computerId}>
            {saving && <LoaderCircle className="size-4 animate-spin" />} Salvar agendamento
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
