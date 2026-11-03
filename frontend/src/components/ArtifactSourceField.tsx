import { useId } from "react"
import { cn } from "@/lib/utils"
import { Input } from "@/components/ui/input"

export type ArtifactSource = { kind: "link" | "file"; url: string; file?: File }

export const MAX_ARTIFACT_BYTES = 16 * 1024 * 1024

// sourceError returns what is wrong with the source before anything is sent, or "" when it is fine.
export function sourceError(v: ArtifactSource): string {
  if (v.kind === "link") {
    const url = v.url.trim()
    if (!url) return ""
    return /^https:\/\//i.test(url) || /^http:\/\/localhost(?=[:/]|$)/i.test(url) ? "" : "Cole um link que comece com https://."
  }
  if (!v.file) return ""
  if (!/\.html?$/i.test(v.file.name)) return "Envie um arquivo .html."
  if (v.file.size > MAX_ARTIFACT_BYTES) return "O arquivo passa de 16 MB."
  return ""
}

// sourceReady is true once there is something valid to send.
export const sourceReady = (v: ArtifactSource) => (v.kind === "link" ? v.url.trim() !== "" : !!v.file) && sourceError(v) === ""

export function readAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onerror = () => reject(new Error("Não consegui ler o arquivo."))
    r.onload = () => {
      const s = String(r.result)
      resolve(s.slice(s.indexOf(",") + 1))
    }
    r.readAsDataURL(file)
  })
}

export function ArtifactSourceField({ value, onChange }: { value: ArtifactSource; onChange: (v: ArtifactSource) => void }) {
  const id = useId()
  const error = sourceError(value)
  const tab = (kind: "link" | "file", label: string) => (
    <button
      type="button"
      role="tab"
      aria-selected={value.kind === kind}
      onClick={() => onChange({ ...value, kind })}
      className={cn(
        "rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
        value.kind === kind ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
      )}
    >
      {label}
    </button>
  )
  return (
    <div className="flex flex-col gap-2">
      <div role="tablist" aria-label="Origem do artifact" className="inline-flex w-fit gap-1 rounded-lg bg-muted p-1">
        {tab("link", "Colar link")}
        {tab("file", "Subir arquivo")}
      </div>
      {value.kind === "link" ? (
        <>
          <label htmlFor={`${id}-url`} className="sr-only">
            Link
          </label>
          <Input
            id={`${id}-url`}
            type="url"
            inputMode="url"
            value={value.url}
            onChange={(e) => onChange({ ...value, url: e.target.value })}
            placeholder="https://claude.ai/…"
            aria-invalid={!!error}
          />
        </>
      ) : (
        <>
          <label htmlFor={`${id}-file`} className="sr-only">
            Arquivo HTML
          </label>
          <input
            id={`${id}-file`}
            type="file"
            accept=".html,.htm"
            onChange={(e) => onChange({ ...value, file: e.target.files?.[0] })}
            className="block w-full cursor-pointer rounded-lg border border-input bg-card text-sm text-muted-foreground file:mr-3 file:cursor-pointer file:border-0 file:bg-accent file:px-3 file:py-2 file:text-sm file:font-medium file:text-foreground"
          />
        </>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
