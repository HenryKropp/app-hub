import { SHARED_STATUS } from "@/lib/shared"
import type { SharedStatus } from "@/lib/api"
import { cn } from "@/lib/utils"

const TONE = {
  ok: "bg-ok",
  live: "bg-brand animate-pulse",
  muted: "bg-muted-foreground/50",
  bad: "bg-destructive",
  draft: "border border-muted-foreground/60 bg-transparent",
} as const

// label replaces the short status word (the ficha shows the longer sharedLine).
export function SharedStatusDot({ status, className, label }: { status: SharedStatus; className?: string; label?: string }) {
  const s = SHARED_STATUS[status]
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-xs", className)}>
      <span aria-hidden className={cn("size-2 rounded-full", TONE[s.tone])} />
      {label ?? s.label}
    </span>
  )
}
