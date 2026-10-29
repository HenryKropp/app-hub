import { BrowserRouter, Navigate, Route, Routes, useLocation } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { Layout } from "@/components/Layout"
import { AuthProvider, useAuth } from "@/lib/auth"
import { ThemeProvider } from "@/lib/theme"
import { AppPage } from "@/pages/AppPage"
import { CatalogPage } from "@/pages/CatalogPage"
import { ConnectPage } from "@/pages/ConnectPage"
import { LoginPage } from "@/pages/LoginPage"
import { canRegister } from "@/lib/permissions"
import { ServerPage } from "@/pages/ServerPage"
import { TeamPage } from "@/pages/TeamPage"
import { RegisterPage } from "@/pages/RegisterPage"
import { MyRunsPage } from "@/pages/MyRunsPage"
import { RunPage } from "@/pages/RunPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { VersionPage } from "@/pages/VersionPage"
import { SetupPage } from "@/pages/SetupPage"
import { PRODUCT_NAME } from "@/lib/brand"

function Protected() {
  const { loading, person } = useAuth()
  const location = useLocation()
  if (loading) return <div className="mx-auto mt-20 w-full max-w-sm space-y-3 px-4" role="status" aria-label="Verificando acesso"><div className="h-7 w-2/3 animate-pulse rounded bg-muted" /><div className="h-4 w-full animate-pulse rounded bg-muted" /><div className="h-10 w-full animate-pulse rounded bg-muted" /></div>
  if (!person) return <Navigate to="/entrar" replace state={{ from: location.pathname + location.search }} />
  return <Layout />
}

function CreatorsOnly({ children }: { children: React.ReactNode }) {
  const { person } = useAuth()
  if (!canRegister(person)) return <Navigate to="/" replace />
  return children
}

function Routed() {
  const { loading } = useAuth()
  if (loading) return <div className="mx-auto mt-20 w-full max-w-sm space-y-3 px-4" role="status" aria-label={`Conectando ao ${PRODUCT_NAME}`}><div className="h-7 w-2/3 animate-pulse rounded bg-muted" /><div className="h-4 w-full animate-pulse rounded bg-muted" /><div className="h-10 w-full animate-pulse rounded bg-muted" /></div>
  return (
    <Routes>
      <Route path="/entrar" element={<LoginPage />} />
      <Route element={<Protected />}>
        <Route index element={<CatalogPage />} />
        <Route path="apps/:id" element={<AppPage />} />
        <Route path="apps/:id/versoes/:n" element={<VersionPage />} />
        <Route
          path="registrar"
          element={
            <CreatorsOnly>
              <RegisterPage />
            </CreatorsOnly>
          }
        />
        <Route path="equipe" element={<TeamPage />} />
        <Route path="servidor" element={<ServerPage />} />
        <Route path="pessoas" element={<Navigate to="/equipe" replace />} />
        <Route path="configuracoes" element={<SettingsPage />} />
        <Route path="comecar" element={<SetupPage />} />
        <Route path="meu-computador" element={<Navigate to="/configuracoes" replace />} />
        <Route path="conectar" element={<ConnectPage />} />
        <Route path="execucoes" element={<MyRunsPage />} />
        <Route path="execucoes/:id" element={<RunPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <BrowserRouter>
          <Routed />
        </BrowserRouter>
        <Toaster position="bottom-right" />
      </AuthProvider>
    </ThemeProvider>
  )
}
