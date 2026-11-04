import type { ReactNode } from "react"
import { Wordmark } from "@/components/bits"
import { cn } from "@/lib/utils"

/** Links on the public pages: ink with an underline, never coral. */
export const authLink = "font-medium text-foreground underline underline-offset-4 decoration-foreground/30 transition-colors hover:decoration-foreground"

/** The quiet link back to where the person came from, top left. */
export const authBack = "inline-flex items-center gap-1.5 rounded-md text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"

/** An error a person can read and act on. */
export const authError = "rounded-lg bg-destructive/10 px-3 py-2 text-sm leading-relaxed text-destructive"

/**
 * The shell of the sign-in pages: white ground, the Ziv logo, one centered card.
 * `logo` lets each page decide whether the logo links home (a workspace sign-in does not).
 */
export function AuthLayout({ back, logo, below, wide, children }: {
  back?: ReactNode
  logo?: ReactNode
  below?: ReactNode
  wide?: boolean
  children: ReactNode
}) {
  return (
    <main className="flex min-h-svh flex-col bg-background text-foreground">
      <header className="flex h-14 shrink-0 items-center px-4 sm:px-6">{back}</header>
      <div className="flex flex-1 flex-col items-center px-4 pb-14 pt-2 sm:pt-8">
        {logo ?? <Wordmark className="h-10 w-auto" />}
        <div className={cn("entry-rise mt-7 w-full rounded-xl border border-border bg-card p-6 sm:p-8", wide ? "max-w-md" : "max-w-sm")}>
          {children}
        </div>
        {below && <div className={cn("mt-6 w-full text-center text-sm text-muted-foreground", wide ? "max-w-md" : "max-w-sm")}>{below}</div>}
      </div>
    </main>
  )
}
