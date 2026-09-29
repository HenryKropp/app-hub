import type { ComponentProps, ReactNode } from "react"
import { Monitor, Moon, Sun } from "lucide-react"
import type { Kind } from "@/lib/api"
import { kindInfo } from "@/lib/kinds"
import { useTheme, type ThemePref } from "@/lib/theme"
import { cn } from "@/lib/utils"

export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true">
      <path d="M4 9h9l2-3h13v20H4z" fill="currentColor" opacity="0.18" />
      <path d="M4 11h24v17H4z" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M4 11V8h9l2-3h7" fill="none" stroke="currentColor" strokeWidth="2" />
      <circle cx="22" cy="21" r="3.5" fill="var(--stamp)" />
      <path d="M8 17h8M8 21h6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}

/** The colored folder tab that marks each app's kind. */
export function KindTab({ kind, className }: { kind: Kind; className?: string }) {
  const k = kindInfo(kind)
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-t-md rounded-b-sm px-2 py-0.5 font-mono text-[0.68rem] font-medium tracking-wide uppercase",
        className,
      )}
      style={{ background: `color-mix(in oklch, ${k.color} 22%, transparent)`, color: `color-mix(in oklch, ${k.color} 70%, var(--foreground))` }}
    >
      <span className="size-1.5 rounded-full" style={{ background: k.color }} />
      {k.short}
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
      <label htmlFor={htmlFor} className="text-[0.8rem] font-semibold tracking-wide text-foreground/85">
        {label}
      </label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

export function PageHeader({ eyebrow, title, children }: { eyebrow?: string; title: string; children?: ReactNode }) {
  return (
    <header className="rise mb-8 flex flex-col gap-3 border-b border-border pb-6 md:flex-row md:items-end md:justify-between">
      <div>
        {eyebrow && <p className="mb-2 font-mono text-xs tracking-[0.18em] text-stamp uppercase">{eyebrow}</p>}
        <h1 className="font-heading text-3xl leading-tight md:text-4xl">{title}</h1>
      </div>
      {children}
    </header>
  )
}

const THEME_OPTIONS: { value: ThemePref; label: string; icon: typeof Sun }[] = [
  { value: "light", label: "Claro", icon: Sun },
  { value: "dark", label: "Escuro", icon: Moon },
  { value: "system", label: "Sistema", icon: Monitor },
]

export function ThemeToggle() {
  const { pref, setPref } = useTheme()
  return (
    <div className="flex rounded-md bg-white/5 p-0.5" role="radiogroup" aria-label="Tema">
      {THEME_OPTIONS.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={pref === value}
          title={label}
          onClick={() => setPref(value)}
          className={cn(
            "flex flex-1 items-center justify-center gap-1 rounded px-2 py-1 text-[0.7rem] transition-colors",
            pref === value ? "bg-ink-foreground/15 text-ink-foreground" : "text-ink-foreground/55 hover:text-ink-foreground",
          )}
        >
          <Icon className="size-3.5" />
          <span className="sr-only md:not-sr-only">{label}</span>
        </button>
      ))}
    </div>
  )
}

export function EmptyState({ title, children, action }: { title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="rise paper flex flex-col items-start gap-3 rounded-lg border border-dashed border-border bg-card/60 py-12 pr-8 pl-20">
      <h2 className="font-heading text-2xl">{title}</h2>
      {children && <div className="max-w-prose text-muted-foreground">{children}</div>}
      {action}
    </div>
  )
}
