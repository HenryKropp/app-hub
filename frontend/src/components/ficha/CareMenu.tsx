import type { ReactNode } from "react"
import { Check } from "lucide-react"
import { cn } from "@/lib/utils"

/** The two faces of an app's page: using it, and taking care of it (ADR 029). */
export type Face = "usar" | "cuidar"

export function FaceSwitch({ face, onChange, pending }: { face: Face; onChange: (f: Face) => void; pending: number }) {
  const item = (f: Face, label: ReactNode) => (
    <button
      type="button"
      role="tab"
      aria-selected={face === f}
      onClick={() => onChange(f)}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm text-muted-foreground transition-colors",
        face === f ? "bg-background font-medium text-foreground shadow-xs" : "hover:text-foreground",
      )}
    >
      {label}
    </button>
  )
  return (
    <div role="tablist" aria-label="Como ver o app" className="inline-flex rounded-full bg-secondary p-1">
      {item("usar", "Usar")}
      {item(
        "cuidar",
        <>
          Cuidar
          {pending > 0 && <CountBadge n={pending} tone="warn" label={`${pending} ${pending === 1 ? "pendência" : "pendências"}`} />}
        </>,
      )}
    </div>
  )
}

function CountBadge({ n, tone, label }: { n: number; tone: "warn" | "quiet"; label: string }) {
  return (
    <span
      aria-label={label}
      className={cn(
        "rounded-full px-1.5 text-[11px] leading-4 tabular-nums",
        tone === "warn" ? "border border-warn/35 bg-warn-soft text-warn" : "bg-secondary text-muted-foreground",
      )}
    >
      {n}
    </span>
  )
}

export type MenuItem = { key: string; label: string; group?: string; count?: number; warn?: boolean }

/** The menu of the Cuidar face: groups of sections, one shown at a time. */
export function CareNav({ items, current, onPick }: { items: MenuItem[]; current: string; onPick: (k: string) => void }) {
  let lastGroup: string | undefined
  return (
    <nav aria-label="Seções de cuidado" className="flex gap-0.5 overflow-x-auto pb-1 md:flex-col md:overflow-visible md:pb-0">
      {items.map((it) => {
        const head = it.group && it.group !== lastGroup
        lastGroup = it.group
        return (
          <div key={it.key} className="contents">
            {head && (
              <p className="hidden px-2.5 pt-4 pb-1 text-[11px] font-medium tracking-wider text-muted-foreground uppercase md:block">{it.group}</p>
            )}
            <button
              type="button"
              aria-current={current === it.key ? "page" : undefined}
              onClick={() => onPick(it.key)}
              className={cn(
                "flex shrink-0 items-center justify-between gap-3 rounded-md px-2.5 py-1.5 text-left text-sm whitespace-nowrap text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground",
                current === it.key && "bg-secondary font-medium text-foreground",
              )}
            >
              {it.label}
              {!!it.count && <CountBadge n={it.count} tone={it.warn ? "warn" : "quiet"} label={String(it.count)} />}
            </button>
          </div>
        )
      })}
    </nav>
  )
}

export type Step = { label: string; done: boolean; state: string; bad?: boolean }

/** Where the app stands, left to right: registrado → aprovado → publicado → com reserva. */
export function Steps({ steps }: { steps: Step[] }) {
  const firstOpen = steps.findIndex((s) => !s.done)
  return (
    <ol className="flex flex-wrap overflow-hidden rounded-xl border border-border" aria-label="Etapas do app">
      {steps.map((s, i) => (
        <li
          key={s.label}
          aria-current={i === firstOpen ? "step" : undefined}
          className={cn(
            "flex min-w-36 flex-1 flex-col gap-0.5 border-border px-3.5 py-2.5 not-first:border-l",
            i === firstOpen && (s.bad ? "bg-destructive/5" : "bg-warn-soft"),
          )}
        >
          <span className="text-xs text-muted-foreground">{i + 1} · {s.label}</span>
          <span className={cn("flex items-center gap-1 text-sm", s.done ? "text-ok" : s.bad ? "text-destructive" : i === firstOpen ? "text-warn" : "text-muted-foreground")}>
            {s.done && <Check className="size-3.5" />}
            {s.state}
          </span>
        </li>
      ))}
    </ol>
  )
}

export type Todo = { key: string; title: string; detail?: string; action?: ReactNode }

/** One thing to do on the app, with the button that does it on the right. */
export function TodoList({ todos }: { todos: Todo[] }) {
  return (
    <ul className="flex flex-col gap-2">
      {todos.map((t) => (
        <li key={t.key} className="flex flex-col gap-2 rounded-xl border border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm">
            <span className="font-medium">{t.title}</span>
            {t.detail && <span className="text-muted-foreground"> · {t.detail}</span>}
          </p>
          {t.action && <div className="flex shrink-0 gap-2">{t.action}</div>}
        </li>
      ))}
    </ul>
  )
}
