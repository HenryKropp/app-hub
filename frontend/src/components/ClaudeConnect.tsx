import { useEffect, useState, type FormEvent } from "react"
import { Check, Copy, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, workspaceBasePath, type McpCode } from "@/lib/api"

// eslint-disable-next-line react-refresh/only-export-components
export const isLocalHost = (hostname: string) => hostname === "localhost" || hostname === "127.0.0.1"

/** Copies text and says "Copiado" for a moment. If the browser refuses, the text stays on screen to select by hand. */
function CopyButton({ text, label }: { text: string; label: string }) {
  const [copied, setCopied] = useState(false)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    if (!copied) return
    const t = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(t)
  }, [copied])
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setFailed(false)
      setCopied(true)
    } catch {
      setFailed(true)
    }
  }
  return (
    <span className="inline-flex items-center gap-2">
      <Button type="button" variant="outline" size="sm" aria-label={label} onClick={copy} className="gap-1.5">
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
        {copied ? "Copiado" : "Copiar"}
      </Button>
      {failed && <span className="text-xs text-muted-foreground">Não deu para copiar. Selecione o texto e copie.</span>}
    </span>
  )
}

function CopyLine({ text, label }: { text: string; label: string }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <code className="min-w-0 flex-1 rounded-lg border border-border bg-muted/40 px-2.5 py-1.5 font-mono text-xs break-all select-all">{text}</code>
      <CopyButton text={text} label={label} />
    </div>
  )
}

/** How to connect Claude: the connector address (claude.ai / app) and the Claude Code code. Rendered by the Começar page. */
export function ClaudeConnect() {
  const [name, setName] = useState("")
  const [busy, setBusy] = useState(false)
  const [codeError, setCodeError] = useState("")
  // The code lives only in this component's memory: gone on the next code or when the page closes.
  const [created, setCreated] = useState<McpCode | null>(null)

  const address = `${window.location.origin}${workspaceBasePath()}/mcp`
  const local = isLocalHost(window.location.hostname)

  const generate = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim() || busy) return
    setBusy(true)
    setCodeError("")
    setCreated(null)
    try {
      const c = await api.createMcpCode(name.trim())
      setCreated(c)
      setName("")
    } catch (err) {
      setCodeError(err instanceof Error ? err.message : "Não consegui gerar o código. Tente de novo.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <div className="grid gap-4 lg:grid-cols-2">
        <section aria-labelledby="claude-site" className="rounded-xl border border-border bg-card p-5">
          <h3 id="claude-site" className="font-heading text-base">No site ou no app do Claude</h3>
          <p className="mt-1 mb-3 text-sm text-muted-foreground">Endereço do conector:</p>
          <CopyLine text={address} label="Copiar endereço" />
          {local && (
            <p className="mt-3 flex items-start gap-2 text-sm text-warn">
              <TriangleAlert className="mt-0.5 size-4 shrink-0" />
              <span>Esta forma só funciona depois que o Ziv estiver publicado na internet.</span>
            </p>
          )}
          <ol className="mt-4 flex list-decimal flex-col gap-1.5 pl-5 text-sm">
            <li>No Claude, abra Configurações → Conectores → Adicionar conector personalizado.</li>
            <li>Cole o endereço acima e salve.</li>
            <li>Clique em Conectar e entre com a sua conta do Ziv.</li>
          </ol>
          <p className="mt-3 text-sm text-muted-foreground">
            Se a sua empresa usa o Claude em plano de equipe, quem administra o Claude adiciona o Ziv uma vez; depois cada um só clica em Conectar.
          </p>
        </section>

        <section aria-labelledby="claude-code" className="rounded-xl border border-border bg-card p-5">
          <h3 id="claude-code" className="font-heading text-base">No Claude Code</h3>
          <p className="mt-1 mb-3 text-sm text-muted-foreground">Gere um código para cada computador onde você usa o Claude Code.</p>
          <form onSubmit={generate} className="flex flex-wrap items-end gap-2">
            <div className="flex min-w-48 flex-1 flex-col gap-1.5">
              <label htmlFor="claude-code-nome" className="text-xs font-medium">Nome deste código</label>
              <Input id="claude-code-nome" value={name} maxLength={80} placeholder="Ex.: Notebook do escritório" onChange={(e) => setName(e.target.value)} />
            </div>
            <Button type="submit" disabled={!name.trim() || busy}>{busy ? "Gerando…" : "Gerar código"}</Button>
          </form>
          {codeError && <p role="alert" className="mt-3 text-sm text-destructive">{codeError}</p>}
          {created && (
            <div className="mt-4 flex flex-col gap-2">
              <p className="text-sm">Rode este comando no terminal do computador <strong>{created.name}</strong>:</p>
              <CopyLine text={created.command} label="Copiar comando" />
              <p className="flex items-center gap-2 text-sm font-medium text-warn">
                <TriangleAlert className="size-4 shrink-0" /> Guarde agora: o código não aparece de novo.
              </p>
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
