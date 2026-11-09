import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom"
import { History, House, LayoutDashboard, LibraryBig, ListChecks, PanelsTopLeft, LogOut, Server, Settings, Users } from "lucide-react"
import { api, workspaceBasePath, type Person } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { canSeeServer, canSeeTeam, has } from "@/lib/permissions"
import { RunsProvider, useRuns } from "@/lib/runs-context"
import { isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"
import { Playing, Wordmark } from "./bits"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu"
import { RunStrip } from "./RunStrip"
import { AgentBanner } from "./AgentDownload"
import { ComputerStatusProvider } from "@/lib/computer-status"

const NAV = [
  { to: "/", label: "Home", icon: House, show: (_p: Person) => true },
  { to: "/apps", label: "Apps", icon: LibraryBig, show: (_p: Person) => true },
  { to: "/artifacts", label: "Artifacts", icon: PanelsTopLeft, show: (_p: Person) => true },
  { to: "/execucoes", label: "Minhas execuções", icon: History, show: (_p: Person) => true },
  { to: "/equipe", label: "Equipe", icon: Users, show: canSeeTeam },
  { to: "/admin", label: "Admin", icon: LayoutDashboard, show: (p: Person) => has(p, "manage_roles") },
  { to: "/servidor", label: "Servidor", icon: Server, show: canSeeServer },
  { to: "/comecar", label: "Começar", icon: ListChecks, show: (_p: Person) => workspaceBasePath() !== "" },
]

const NAV_ITEM = "flex shrink-0 items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors"
const NAV_ON = "relative bg-accent font-medium text-foreground before:absolute before:inset-y-1.5 before:left-0 before:w-[3px] before:rounded-full before:bg-brand"
const NAV_OFF = "text-muted-foreground hover:bg-accent/70 hover:text-foreground"

export function Layout() {
  const { person } = useAuth()
  if (!person) return null
  return (
    <ComputerStatusProvider>
      <RunsProvider>
        <Shell />
      </RunsProvider>
    </ComputerStatusProvider>
  )
}

const initials = (name: string) =>
  name.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]!.toUpperCase()).join("")

function Shell() {
  const { person, setPerson } = useAuth()
  const navigate = useNavigate()
  // Registrar apps opens from the Apps page's +, so it counts as Apps.
  const path = useLocation().pathname
  const inCatalog = path.startsWith("/registrar")
  // Pages that already are about connecting the computer skip the strip.
  const agentStrip = !["/comecar", "/conectar", "/configuracoes"].includes(path)
  const openCount = useRuns().active.filter((r) => isLive(r.status)).length
  if (!person) return null

  const logout = async () => {
    await api.logout().catch(() => {})
    setPerson(null)
    navigate("/entrar")
  }

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <a href="#conteudo" className="sr-only z-50 rounded-md bg-background px-3 py-2 text-sm font-medium shadow-pop focus:not-sr-only focus:fixed focus:top-3 focus:left-3">
        Pular para o conteúdo
      </a>
      <aside className="relative z-10 flex shrink-0 flex-col border-b border-sidebar-border bg-sidebar md:sticky md:top-0 md:h-svh md:w-60 md:border-r md:border-b-0">
        <div className="flex items-center gap-2.5 px-5 pt-4 pb-3 md:pt-5 md:pb-6">
          <Wordmark className="h-9 w-auto" />
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
                <p className="text-xs text-muted-foreground">{person.role_name}</p>
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => navigate("/configuracoes")}>
                <Settings className="size-4" /> Configurações
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={logout}>
                <LogOut className="size-4" /> Sair
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <nav className="flex gap-0.5 overflow-x-auto px-3 pb-3 [mask-image:linear-gradient(to_right,black_85%,transparent)] md:flex-col md:overflow-visible md:pb-0 md:[mask-image:none]">
          {NAV.filter((n) => n.show(person)).map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) => cn(NAV_ITEM, isActive || (to === "/apps" && inCatalog) ? NAV_ON : NAV_OFF)}
            >
              {({ isActive }) => (
                <>
                  <Icon className={cn("size-4", (isActive || (to === "/apps" && inCatalog)) && "text-foreground")} />
                  {label}
                  {to === "/execucoes" && openCount > 0 && (
                    <span aria-label={`${openCount} ${openCount === 1 ? "aberto" : "abertos"} agora`} className="ml-auto inline-flex items-center gap-1.5 text-xs font-medium text-foreground">
                      <Playing />
                      {openCount}
                    </span>
                  )}
                </>
              )}
            </NavLink>
          ))}
        </nav>

        <div className="mt-auto hidden flex-col gap-2 p-3 md:flex">
          <NavLink to="/configuracoes" className={({ isActive }) => cn(NAV_ITEM, isActive ? NAV_ON : NAV_OFF)}>
            {({ isActive }) => (
              <>
                <Settings className={cn("size-4", isActive && "text-foreground")} />
                Configurações
              </>
            )}
          </NavLink>
          <div className="border-t border-sidebar-border pt-3">
            <div className="flex items-center justify-between gap-2 px-1">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-secondary text-xs font-semibold">{initials(person.name)}</span>
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{person.name}</p>
                  <p className="text-xs text-muted-foreground">{person.role_name}</p>
                </div>
              </div>
              <button type="button" onClick={logout} title="Sair" className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground">
                <LogOut className="size-4" />
                <span className="sr-only">Sair</span>
              </button>
            </div>
          </div>
        </div>
      </aside>

      <main id="conteudo" tabIndex={-1} className="min-w-0 outline-none flex-1 px-4 pt-8 pb-40 sm:px-8 md:px-12 md:pt-10">
        <div className="mx-auto max-w-6xl">
          {agentStrip && <AgentBanner />}
          <Outlet />
        </div>
      </main>
      <RunStrip />
    </div>
  )
}
