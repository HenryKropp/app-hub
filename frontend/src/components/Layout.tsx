import { NavLink, Outlet, useNavigate } from "react-router-dom"
import { FolderSearch, Gauge, History, KeyRound, Laptop, LibraryBig, LogOut, Users } from "lucide-react"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { roleLabel } from "@/lib/kinds"
import { RunsProvider, useRuns } from "@/lib/runs-context"
import { isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"
import { Logo, Playing, ThemeToggle } from "./bits"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu"
import { useTheme, type ThemePref } from "@/lib/theme"
import { RunStrip } from "./RunStrip"

const NAV = [
  { to: "/", label: "Catálogo", icon: LibraryBig, roles: ["admin", "criador", "usuario"] },
  { to: "/execucoes", label: "Minhas execuções", icon: History, roles: ["admin", "criador", "usuario"] },
  { to: "/registrar", label: "Registrar apps", icon: FolderSearch, roles: ["admin", "criador"] },
  { to: "/pessoas", label: "Pessoas", icon: Users, roles: ["admin", "criador", "usuario"] },
  { to: "/meu-computador", label: "Meu computador", icon: Laptop, roles: ["admin", "criador", "usuario"] },
]

const SOON = [
  { label: "Painel", icon: Gauge },
  { label: "Cofre", icon: KeyRound },
]

export function Layout() {
  const { person } = useAuth()
  if (!person) return null
  return (
    <RunsProvider>
      <Shell />
    </RunsProvider>
  )
}

const initials = (name: string) =>
  name.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]!.toUpperCase()).join("")

function Shell() {
  const { person, setPerson } = useAuth()
  const navigate = useNavigate()
  const { pref, setPref } = useTheme()
  const openCount = useRuns().active.filter((r) => isLive(r.status)).length
  if (!person) return null

  const logout = async () => {
    await api.logout().catch(() => {})
    setPerson(null)
    navigate("/entrar")
  }

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside className="relative z-10 flex shrink-0 flex-col border-b border-sidebar-border bg-sidebar md:sticky md:top-0 md:h-svh md:w-60 md:border-r md:border-b-0">
        <div className="flex items-center gap-2.5 px-5 pt-4 pb-3 md:pt-5 md:pb-6">
          <Logo className="size-7" />
          <p className="text-[0.95rem] font-semibold tracking-[-0.01em]">App Hub</p>
          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label="Sua conta"
              className="ml-auto flex size-8 items-center justify-center rounded-full bg-secondary text-xs font-semibold md:hidden"
            >
              {initials(person.name)}
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <div className="px-1.5 py-1.5">
                <p className="truncate text-sm font-medium">{person.name}</p>
                <p className="text-xs text-muted-foreground">{roleLabel(person.role)}</p>
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuRadioGroup value={pref} onValueChange={(v) => setPref(v as ThemePref)}>
                <DropdownMenuRadioItem value="light">Claro</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="dark">Escuro</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="system">Igual ao sistema</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={logout}>
                <LogOut className="size-4" /> Sair
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <nav className="flex gap-0.5 overflow-x-auto px-3 pb-3 [mask-image:linear-gradient(to_right,black_85%,transparent)] md:flex-col md:overflow-visible md:pb-0 md:[mask-image:none]">
          {NAV.filter((n) => n.roles.includes(person.role)).map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                cn(
                  "flex shrink-0 items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors",
                  isActive ? "bg-accent font-medium text-foreground" : "text-muted-foreground hover:bg-accent/70 hover:text-foreground",
                )
              }
            >
              {({ isActive }) => (
                <>
                  <Icon className={cn("size-4", isActive && "text-brand")} />
                  {label}
                  {to === "/execucoes" && openCount > 0 && (
                    <span aria-label={`${openCount} aberto agora`} className="ml-auto inline-flex items-center gap-1.5 text-xs font-medium text-brand">
                      <Playing />
                      {openCount}
                    </span>
                  )}
                </>
              )}
            </NavLink>
          ))}
          <div className="hidden md:mt-6 md:block">
            <p className="px-2.5 pb-1 text-xs text-muted-foreground">Em breve</p>
            {SOON.map(({ label, icon: Icon }) => (
              <div key={label} aria-disabled="true" className="flex items-center gap-2.5 px-2.5 py-1.5 text-sm text-muted-foreground/60">
                <Icon className="size-4" />
                {label}
              </div>
            ))}
          </div>
        </nav>

        <div className="mt-auto hidden flex-col gap-3 border-t border-sidebar-border p-3 md:flex">
          <div className="flex items-center justify-between gap-2 px-1">
            <div className="flex min-w-0 items-center gap-2.5">
              <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-secondary text-xs font-semibold">{initials(person.name)}</span>
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{person.name}</p>
                <p className="text-xs text-muted-foreground">{roleLabel(person.role)}</p>
              </div>
            </div>
            <button type="button" onClick={logout} title="Sair" className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground">
              <LogOut className="size-4" />
              <span className="sr-only">Sair</span>
            </button>
          </div>
          <ThemeToggle />
        </div>
      </aside>

      <main className="min-w-0 flex-1 px-4 pt-8 pb-40 sm:px-8 md:px-12 md:pt-10">
        <div className="mx-auto max-w-6xl">
          <Outlet />
        </div>
      </main>
      <RunStrip />
    </div>
  )
}
