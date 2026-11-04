import { useCallback, useEffect, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { AuthLayout, authError } from "@/components/AuthLayout"
import { api, ApiError, type OAuthRequest } from "@/lib/api"

/** Where Claude's own pages live, plus the local addresses Claude Code uses. Anything else gets a warning. */
// eslint-disable-next-line react-refresh/only-export-components
export function isClaudeHost(host: string) {
  const h = host.toLowerCase().replace(/:\d+$/, "")
  if (h === "localhost" || h === "127.0.0.1") return true
  return ["claude.ai", "claude.com"].some((d) => h === d || h.endsWith(`.${d}`))
}

const EXPIRED = "Esse pedido de conexão venceu. Volte ao Claude e clique em Conectar de novo."
const RETRY = "Tente de novo; se continuar, volte ao Claude e clique em Conectar de novo."

export function ClaudeConsentPage() {
  const [params] = useSearchParams()
  const id = params.get("pedido") ?? ""
  const [req, setReq] = useState<OAuthRequest | null>(null)
  const [loadError, setLoadError] = useState(id ? "" : EXPIRED)
  const [busy, setBusy] = useState<"" | "approve" | "deny">("")
  const [error, setError] = useState("")

  const load = useCallback(() => {
    if (!id) return
    setLoadError("")
    api.oauthRequest(id).then(setReq, (e: Error) => {
      setLoadError(e instanceof ApiError && e.status === 404 ? EXPIRED : e.message)
    })
  }, [id])
  useEffect(() => { load() }, [load])

  const answer = async (choice: "approve" | "deny") => {
    if (busy) return
    setBusy(choice)
    setError("")
    try {
      const { redirect } = choice === "approve" ? await api.approveOauth(id) : await api.denyOauth(id)
      window.location.assign(redirect)
      // Keep the buttons off: the browser is leaving for Claude.
    } catch (e) {
      const msg = e instanceof ApiError && e.status === 404 ? EXPIRED : `${e instanceof Error ? e.message : "Algo deu errado."} ${RETRY}`
      setError(msg)
      setBusy("")
    }
  }

  let body
  if (loadError) {
    body = loadError === EXPIRED ? (
      <>
        <h1 className="font-heading text-xl">Conectar o Claude</h1>
        <p className="mt-3 text-sm leading-relaxed">{EXPIRED}</p>
      </>
    ) : (
      <>
        <h1 className="font-heading text-xl">Conectar o Claude</h1>
        <p role="alert" className={`mt-3 ${authError}`}>{loadError}</p>
        <Button variant="outline" className="mt-4" onClick={load}>Tentar de novo</Button>
      </>
    )
  } else if (!req) {
    body = (
      <div role="status" aria-label="Carregando pedido" className="space-y-3">
        <div className="h-6 w-3/4 animate-pulse rounded bg-muted" />
        <div className="h-4 w-full animate-pulse rounded bg-muted" />
        <div className="h-4 w-2/3 animate-pulse rounded bg-muted" />
      </div>
    )
  } else {
    const trusted = isClaudeHost(req.redirect_host)
    body = (
      <>
        <h1 className="font-heading text-xl leading-snug">
          Permitir que <strong className="font-semibold">{req.client_name}</strong> use o Ziv como você?
        </h1>
        <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
          Ele poderá achar apps e artifacts, rodar apps no seu computador e publicar artifacts, sempre com as suas permissões.
        </p>
        <p className="mt-4 rounded-lg border border-border px-3 py-2 text-sm">
          O acesso será enviado para <strong className="font-mono font-semibold break-all">{req.redirect_host}</strong>.
        </p>
        {!trusted && (
          <p className="mt-3 flex items-start gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm font-medium text-destructive">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            <span>Atenção: este endereço não é do Claude. Só permita se você mesmo começou esta conexão.</span>
          </p>
        )}
        {error && <p role="alert" className={`mt-4 ${authError}`}>{error}</p>}
        <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" size="lg" disabled={busy !== ""} onClick={() => answer("deny")}>Não permitir</Button>
          <Button size="lg" disabled={busy !== ""} onClick={() => answer("approve")}>{busy === "approve" ? "Permitindo…" : "Permitir"}</Button>
        </div>
        <p className="mt-4 text-xs text-muted-foreground">Você pode cancelar esta conexão quando quiser em Configurações → Conectar o Claude.</p>
      </>
    )
  }

  return <AuthLayout wide>{body}</AuthLayout>
}
