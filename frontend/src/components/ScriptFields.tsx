import type { ChangeEvent } from "react"
import { FolderOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api, type Input as AppInput } from "@/lib/api"
import { acceptText, acceptsFile } from "@/lib/accept"

export type SelectedFile = { file: File; value: string }
export const MAX_FILE_BYTES = 10 * 1024 * 1024

export function readBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error(`Não consegui ler o arquivo ${file.name}.`))
    reader.onload = () => {
      const result = String(reader.result ?? "")
      resolve(result.slice(result.indexOf(",") + 1))
    }
    reader.readAsDataURL(file)
  })
}

export function ScriptField({ field, value, onChange, file, onFile, onError, disabled }: {
  field: AppInput
  value: string | string[] | undefined
  onChange: (value: string | string[]) => void
  file?: File
  onFile: (file?: File) => void
  onError: (message: string) => void
  disabled: boolean
}) {
  const id = `run-input-${field.id}`
  const hint = field.source === "argument"
    ? `O Ziv envia este valor como argumento${field.flag === "" ? " posicional" : ` ${field.flag || `--${field.id}`}`}.`
    : field.source === "prompt"
      ? "O Ziv envia este valor ao app quando ele solicitar uma resposta."
      : field.source === "dialog"
        ? `O Ziv disponibiliza o valor para o app em APP_HUB_INPUT_${field.id.toUpperCase()}; o app precisa ler essa variável.`
        : field.source === "fixed_file"
          ? `O arquivo será copiado para ${field.path || "o caminho definido no manifesto"}.`
          : undefined
  if (field.type === "file") {
    const types = field.accept ?? []
    const pick = (input: HTMLInputElement) => {
      const chosen = input.files?.[0]
      if (chosen && !acceptsFile(types, chosen.name)) {
        input.value = ""
        onFile(undefined)
        onError(`${field.label} aceita só arquivos ${acceptText(types)}. O arquivo ${chosen.name} não serve.`)
        return
      }
      onFile(chosen)
    }
    return <Field label={field.label} htmlFor={id} hint={`${types.length ? `Aceita ${acceptText(types)}. ` : ""}${hint ? `${hint} ` : ""}Até 10 MB no total por execução.`}>
      <Input id={id} type="file" accept={types.length ? types.join(",") : undefined} className="h-auto min-h-10 bg-background py-2 file:mr-3 file:rounded file:border-0 file:bg-secondary file:px-2 file:py-1 file:text-xs file:font-medium" disabled={disabled} required={field.required} onChange={(e) => pick(e.target)} />
      {file && <p className="text-xs text-muted-foreground">{file.name}</p>}
      {file && <button type="button" className="mt-1 text-xs text-muted-foreground underline" onClick={() => onFile(undefined)}>Remover arquivo</button>}
    </Field>
  }
  if (field.type === "folder") {
    return <Field label={field.label} htmlFor={id} hint={hint || "Escolha uma pasta no computador."}>
      <div className="flex gap-2">
        <Input id={id} value={typeof value === "string" ? value : ""} onChange={(e) => onChange(e.target.value)} placeholder="Caminho da pasta" required={field.required} disabled={disabled} />
        <Button type="button" variant="outline" className="shrink-0 gap-2" disabled={disabled} onClick={async () => {
          try {
            const result = await api.chooseFolder(typeof value === "string" ? value : "")
            if (!result.canceled) onChange(result.folder)
          } catch (e) {
            onError(e instanceof Error ? e.message : "Não consegui abrir a janela de pastas.")
          }
        }}><FolderOpen className="size-4" /> Escolher</Button>
      </div>
    </Field>
  }
  if (field.type === "choice") {
    const options = field.options ?? []
    if (field.multiple) {
      const selected = Array.isArray(value) ? value : []
      return <fieldset className="min-w-0">
        <legend className="mb-1.5 text-sm font-medium">{field.label}{field.required && <span aria-hidden="true"> *</span>}</legend>
        <div className="flex flex-wrap gap-2">{options.map((option) => <label key={option} className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm">
          <input type="checkbox" disabled={disabled} checked={selected.includes(option)} onChange={(e) => onChange(e.target.checked ? [...selected, option] : selected.filter((item) => item !== option))} />
          {option}
        </label>)}</div>
        {field.required && selected.length === 0 && <span className="sr-only">Selecione pelo menos uma opção.</span>}
      </fieldset>
    }
    return <Field label={field.label} htmlFor={id} hint={hint}>
      <select id={id} value={typeof value === "string" ? value : ""} onChange={(e) => onChange(e.target.value)} required={field.required} disabled={disabled} className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm">
        <option value="">Selecione</option>{options.map((option) => <option key={option} value={option}>{option}</option>)}
      </select>
    </Field>
  }
  return <Field label={field.label} htmlFor={id} hint={hint}>
    <Input id={id} type={field.type} value={typeof value === "string" ? value : ""} onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)} required={field.required} disabled={disabled} />
  </Field>
}
