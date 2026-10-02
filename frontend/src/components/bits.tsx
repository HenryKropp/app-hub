import type { ComponentProps, ReactNode } from "react"
import { AppWindow, Box, CalendarClock, CircleCheck, FileCode2, FileText, Globe, Info, Monitor, Moon, Network, Sun, TriangleAlert, type LucideIcon } from "lucide-react"
import type { Kind, RunStatus } from "@/lib/api"
import { STATUS } from "@/lib/runs"
import { kindInfo } from "@/lib/kinds"
import { useTheme, type ThemePref } from "@/lib/theme"
import { cn } from "@/lib/utils"

export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true">
      <rect width="32" height="32" rx="8" fill="var(--brand)" />
      <rect x="7" y="7" width="8" height="8" rx="2" fill="#fff" />
      <rect x="17" y="7" width="8" height="8" rx="2" fill="#fff" opacity="0.6" />
      <rect x="7" y="17" width="8" height="8" rx="2" fill="#fff" opacity="0.6" />
      <path d="M18 17.4v7.2a.6.6 0 0 0 .9.5l6-3.6a.6.6 0 0 0 0-1l-6-3.6a.6.6 0 0 0-.9.5z" fill="#fff" />
    </svg>
  )
}

const KIND_ICON: Record<Kind, LucideIcon> = {
  script: FileCode2,
  web: Globe,
  desktop: AppWindow,
  scheduled: CalendarClock,
  executable: Box,
  reference: FileText,
  system: Network,
}

/** A quiet gray pill that names the app's kind. Kinds are never colored: color is for Rodar and what is running. */
export function KindTab({ kind, className }: { kind: Kind; className?: string }) {
  const k = kindInfo(kind)
  const Icon = KIND_ICON[kind] ?? FileCode2
  return (
    <span className={cn("inline-flex items-center gap-1 rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-muted-foreground", className)}>
      <Icon className="size-3.5" />
      {k.short}
    </span>
  )
}

/** The "playing now" bars: the signature mark for whatever is open on someone's computer. */
export function Playing({ className, label }: { className?: string; label?: string }) {
  return (
    <span className={cn("playing text-brand", className)} role={label ? "img" : undefined} aria-label={label} aria-hidden={label ? undefined : true}>
      <i />
      <i />
      <i />
    </span>
  )
}

export function NativeSelect({ className, ...props }: ComponentProps<"select">) {
  return (
    <select
      className={cn(
        "h-9 w-full rounded-md border border-input bg-card px-2.5 text-sm shadow-xs outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/40 disabled:opacity-50",
        className,
      )}
      {...props}
    />
  )
}

export function Field({ label, hint, htmlFor, children, className }: { label: ReactNode; hint?: ReactNode; htmlFor?: string; children: ReactNode; className?: string }) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <label htmlFor={htmlFor} className="text-sm font-medium text-foreground">
        {label}
      </label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

// folderName is the last part of a Windows path: D:\Saídas\CVM → CVM.
// eslint-disable-next-line react-refresh/only-export-components
export const folderName = (p: string) => p.replace(/[\\/]+$/, "").split(/[\\/]/).pop() || p

// SavedIn says where a run's files go. The short form shows the folder's name
// (whole path on hover) and stays on one line; full shows the whole path.
// onChange adds "trocar".
export function SavedIn({ folder, full, onChange, className }: { folder: string; full?: boolean; onChange?: () => void; className?: string }) {
  const change = onChange && (
    <button type="button" onClick={onChange} className="relative z-10 shrink-0 underline underline-offset-4 hover:text-foreground">
      trocar
    </button>
  )
  if (full)
    return (
      <span className={cn("min-w-0 text-muted-foreground", className)}>
        Salva em <span className="font-medium break-all text-foreground/80">{folder}</span>
        {change && <> · {change}</>}
      </span>
    )
  return (
    <span className={cn("flex min-w-0 items-baseline gap-1 text-muted-foreground", className)}>
      <span className="shrink-0">Salva em</span>
      <span title={folder} className="truncate font-medium text-foreground/80">{folderName(folder)}</span>
      {change && <><span aria-hidden="true">·</span>{change}</>}
    </span>
  )
}

export function PageHeader({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <header className="mb-8 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
      <h1 className="font-heading text-2xl leading-tight md:text-[1.75rem]">{title}</h1>
      {children}
    </header>
  )
}

const THEME_OPTIONS: { value: ThemePref; label: string; icon: typeof Sun }[] = [
  { value: "light", label: "Claro", icon: Sun },
  { value: "dark", label: "Escuro", icon: Moon },
  { value: "system", label: "Igual ao sistema", icon: Monitor },
]

export function ThemeToggle({ size = "sm" }: { size?: "sm" | "lg" }) {
  const { pref, setPref } = useTheme()
  return (
    <div className="flex rounded-lg bg-secondary p-0.5" role="radiogroup" aria-label="Tema">
      {THEME_OPTIONS.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={pref === value}
          onClick={() => setPref(value)}
          className={cn(
            "flex flex-auto items-center justify-center gap-1.5 whitespace-nowrap rounded-md transition-colors",
            size === "lg" ? "px-3 py-1.5 text-sm" : "px-2 py-1 text-xs",
            pref === value ? "bg-background text-foreground shadow-xs" : "text-muted-foreground hover:text-foreground",
          )}
        >
          <Icon className={size === "lg" ? "size-4" : "size-3.5"} />
          {label}
        </button>
      ))}
    </div>
  )
}

export function EmptyState({ title, children, action }: { title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-start gap-3 rounded-xl border border-dashed border-input px-8 py-12">
      <h2 className="font-heading text-xl">{title}</h2>
      {children && <div className="max-w-prose text-muted-foreground">{children}</div>}
      {action}
    </div>
  )
}

/** A run's state as icon plus words. Only a live run is violet; the rest stay quiet. */
export function RunState({ status, className }: { status: RunStatus; className?: string }) {
  const st = STATUS[status]
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-medium",
        st.tone === "live" && "text-brand",
        st.tone === "ok" && "text-ok",
        st.tone === "muted" && "text-muted-foreground",
        st.tone === "bad" && "text-destructive",
        className,
      )}
    >
      {st.tone === "live" && <Playing />}
      {st.tone === "ok" && <CircleCheck className="size-4" />}
      {st.tone === "muted" && <Info className="size-4" />}
      {st.tone === "bad" && <TriangleAlert className="size-4" />}
      {st.label}
    </span>
  )
}
