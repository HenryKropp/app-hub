import { NavLink, Outlet, useNavigate } from "react-router-dom"
import { FolderSearch, Gauge, KeyRound, Laptop, LibraryBig, LogOut, Users } from "lucide-react"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { roleLabel } from "@/lib/kinds"
import { cn } from "@/lib/utils"
import { Logo, ThemeToggle } from "./bits"

const NAV = [
  { to: "/", label: "Catálogo", icon: LibraryBig, roles: ["admin", "criador", "usuario"] },
  { to: "/registrar", label: "Registrar apps", icon: FolderSearch, roles: ["admin", "criador"] },
  { to: "/pessoas", label: "Pessoas", icon: Users, roles: ["admin", "criador", "usuario"] },
  { to: "/meu-computador", label: "Meu computador", icon: Laptop, roles: ["admin", "criador", "usuario"] },
]

const SOON = [
  { label: "Painel", icon: Gauge },
  { label: "Cofre", icon: KeyRound },
]

export function Layout() {
  const { person, setPerson } = useAuth()
  const navigate = useNavigate()
  if (!person) return null

  const logout = async () => {
    await api.logout().catch(() => {})
    setPerson(null)
    navigate("/entrar")
  }

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside className="grain relative z-10 flex shrink-0 flex-col bg-ink text-ink-foreground md:sticky md:top-0 md:h-svh md:w-64">
        <div className="flex items-center gap-3 px-5 pt-5 pb-4 md:pt-7 md:pb-8">
          <Logo className="size-9 text-ink-foreground" />
          <div>
            <p className="font-heading text-xl leading-none">App Hub</p>
            <p className="mt-1 font-mono text-[0.65rem] tracking-[0.16em] text-ink-foreground/50 uppercase">fichário de apps</p>
          </div>
        </div>

        <nav className="flex gap-1 overflow-x-auto px-3 pb-3 md:flex-col md:overflow-visible md:pb-0">
          {NAV.filter((n) => n.roles.includes(person.role)).map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                cn(
                  "group relative flex shrink-0 items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                  isActive ? "bg-ink-foreground/10 text-ink-foreground" : "text-ink-foreground/65 hover:bg-ink-foreground/5 hover:text-ink-foreground",
                )
              }
            >
              {({ isActive }) => (
                <>
                  <span className={cn("absolute top-2 bottom-2 -left-3 hidden w-1 rounded-r bg-stamp transition-opacity md:block", isActive ? "opacity-100" : "opacity-0")} />
                  <Icon className="size-4" />
                  {label}
                </>
              )}
            </NavLink>
          ))}
          <div className="hidden md:mt-6 md:block">
            <p className="px-3 pb-2 font-mono text-[0.62rem] tracking-[0.16em] text-ink-foreground/35 uppercase">Em breve</p>
            {SOON.map(({ label, icon: Icon }) => (
              <div key={label} aria-disabled="true" className="flex items-center gap-3 px-3 py-2 text-sm text-ink-foreground/30">
                <Icon className="size-4" />
                {label}
              </div>
            ))}
          </div>
        </nav>

        <div className="mt-auto hidden flex-col gap-3 border-t border-ink-foreground/10 p-4 md:flex">
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">{person.name}</p>
              <p className="text-xs text-ink-foreground/50">{roleLabel(person.role)}</p>
            </div>
            <button type="button" onClick={logout} title="Sair" className="rounded p-1.5 text-ink-foreground/60 hover:bg-ink-foreground/10 hover:text-ink-foreground">
              <LogOut className="size-4" />
              <span className="sr-only">Sair</span>
            </button>
          </div>
          <ThemeToggle />
        </div>
      </aside>

      <main className="min-w-0 flex-1 px-4 py-8 sm:px-8 md:px-12 md:py-12">
        <div className="mx-auto max-w-6xl">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
