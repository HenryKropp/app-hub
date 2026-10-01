import type { DiffLine } from "@/lib/api"

const tone: Record<DiffLine["op"], string> = {
  " ": "",
  "-": "bg-destructive/10 text-destructive",
  "+": "bg-ok-soft text-ok",
}

export function DiffView({ hunks }: { hunks: { lines: DiffLine[] }[] }) {
  return (
    <div className="overflow-x-auto rounded-md border border-border font-mono text-xs">
      {hunks.map((h, i) => (
        <div key={i} className={i > 0 ? "border-t border-dashed border-border" : ""}>
          {h.lines.map((l, j) => (
            <div key={j} data-op={l.op} className={`flex whitespace-pre ${tone[l.op]}`}>
              <span className="w-10 shrink-0 px-1 text-right text-muted-foreground select-none">{l.old || ""}</span>
              <span className="w-10 shrink-0 px-1 text-right text-muted-foreground select-none">{l.new || ""}</span>
              <span className="w-4 shrink-0 select-none">{l.op}</span>
              <span>{l.text}</span>
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
