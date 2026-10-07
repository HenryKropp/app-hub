import { ExternalLink } from "lucide-react"
import { SharedStatusDot } from "@/components/SharedStatus"
import { Button, buttonVariants } from "@/components/ui/button"
import { openSharedUrl, type AppDetail, type SharedInfo } from "@/lib/api"
import { SHARED_STATUS, sharedLine } from "@/lib/shared"
import { cn } from "@/lib/utils"

/** The short line everyone who sees the app gets: how it is, and Abrir once it was published. */
export function SharedLine({ app, info, onRestart, busy }: { app: AppDetail; info?: SharedInfo | null; onRestart?: () => void; busy?: boolean }) {
  const brief = app.shared
  const state = info?.shared ?? null
  const status = state?.status ?? brief?.status
  if (!status) return null
  const line = state ? sharedLine(state, info?.live_version) : SHARED_STATUS[status].label
  const slug = brief?.slug
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm">
      <SharedStatusDot status={status} label={line} className="text-sm" />
      {status !== "draft" && slug && (
        <a href={openSharedUrl(slug)} target="_blank" rel="noopener" aria-label={`Abrir ${app.manifest.name}`}
          className={cn(buttonVariants({ variant: "run" }), "h-8 gap-1.5 pr-3.5 pl-3 [&_svg]:fill-none")}>
          <ExternalLink className="size-3.5" /> Abrir
        </a>
      )}
      {status === "crashed" && onRestart && (
        <Button size="sm" variant="outline" onClick={onRestart} disabled={busy}>Religar</Button>
      )}
    </div>
  )
}
