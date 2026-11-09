import { formatElapsed } from "@/lib/runs"
import { useNow } from "@/lib/use-now"
import { cn } from "@/lib/utils"

/** A timer that ticks every second: how long since `from`. */
export function Elapsed({ from, className }: { from: string; className?: string }) {
  const now = useNow()
  return <span className={cn("font-mono tabular-nums", className)}>{formatElapsed(from, now)}</span>
}
