import { FlaskConical } from "lucide-react"
import { Link } from "react-router-dom"
import type { OpenDraft } from "@/lib/api"

/** The strip on top of the app page while a new version is under test (ADR 044). */
export function DraftBanner({ appId, draft }: { appId: number; draft: OpenDraft }) {
  const label = draft.stage_label || "aprovada, indo para o ar"
  return (
    <div role="status" className="mb-6 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-xl border border-border bg-card px-4 py-3 text-sm">
      <FlaskConical className="size-4 shrink-0 text-muted-foreground" aria-hidden />
      <span className="min-w-0 flex-1">{`Versão nova em teste: ${label}`}</span>
      {draft.stage === "esperando_confirmacao" && (
        <Link to={`/apps/${appId}/plano-de-teste`} className="shrink-0 font-medium underline underline-offset-4 hover:no-underline">
          Confirmar plano de teste
        </Link>
      )}
    </div>
  )
}
