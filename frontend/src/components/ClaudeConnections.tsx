import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { api, type McpConnection } from "@/lib/api"
import { formatWhen } from "@/lib/runs"

/** Settings: only the Claude connections that exist, each with its last use and an undo. Connecting happens in Começar. */
export function ClaudeConnections() {
  const [connections, setConnections] = useState<McpConnection[] | null>(null)
  const [loadError, setLoadError] = useState("")
  const [listError, setListError] = useState("")

  useEffect(() => {
    api.mcpConnections().then((rows) => { setConnections(rows); setLoadError("") }, (e: Error) => setLoadError(e.message))
  }, [])

  const disconnect = async (c: McpConnection) => {
    const label = c.kind === "oauth" ? "a conexão do claude.ai" : `o código ${c.name}`
    if (!window.confirm(`Desconectar ${label}? O Claude perde o acesso ao Ziv por ela na hora.`)) return
    setListError("")
    try {
      await api.revokeMcpConnection(c.id)
      setConnections((rows) => rows?.filter((r) => r.id !== c.id) ?? rows)
    } catch (err) {
      setListError(`Não consegui desconectar: ${err instanceof Error ? err.message : "tente de novo."}`)
    }
  }

  return (
    <section aria-labelledby="claude-titulo">
      <h2 id="claude-titulo" className="font-heading text-lg">Claude</h2>
      <p className="mt-1 mb-5 max-w-[68ch] text-muted-foreground">Onde o Claude está conectado ao Ziv.</p>
      {loadError ? (
        <p role="alert" className="text-sm text-destructive">{loadError}</p>
      ) : connections === null ? (
        <p className="text-sm text-muted-foreground">Carregando…</p>
      ) : connections.length === 0 ? (
        <div className="rounded-xl border border-border bg-card p-6">
          <h3 className="font-heading text-lg">Nenhuma conexão do Claude</h3>
          <Link to="/comecar#claude" className="mt-2 inline-block text-sm underline underline-offset-4">Conectar no Começar</Link>
        </div>
      ) : (
        <ul aria-label="Conexões do Claude" className="divide-y divide-border rounded-xl border border-border bg-card">
          {connections.map((c) => (
            <li key={c.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <span className="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-muted-foreground">{c.kind === "oauth" ? "claude.ai" : "código"}</span>
              <div className="min-w-0 flex-1">
                <p className="font-medium">{c.name}</p>
                <p className="text-sm text-muted-foreground">{c.last_used_at ? `Último uso: ${formatWhen(c.last_used_at)}` : "Nunca usado"}</p>
              </div>
              <Button variant="outline" size="sm" onClick={() => disconnect(c)}>Desconectar</Button>
            </li>
          ))}
        </ul>
      )}
      {listError && <p role="alert" className="mt-2 text-sm text-destructive">{listError}</p>}
    </section>
  )
}
