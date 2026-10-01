import { BrowserRouter, Navigate, Route, Routes, useLocation } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { Layout } from "@/components/Layout"
import { AuthProvider, useAuth } from "@/lib/auth"
import { ThemeProvider } from "@/lib/theme"
import { AppPage } from "@/pages/AppPage"
import { CatalogPage } from "@/pages/CatalogPage"
import { ComputerPage } from "@/pages/ComputerPage"
import { ConnectPage } from "@/pages/ConnectPage"
import { LoginPage } from "@/pages/LoginPage"
import { PeoplePage } from "@/pages/PeoplePage"
import { RegisterPage } from "@/pages/RegisterPage"
import { MyRunsPage } from "@/pages/MyRunsPage"
import { RunPage } from "@/pages/RunPage"
import { VersionPage } from "@/pages/VersionPage"

function Protected() {
  const { loading, person } = useAuth()
  const location = useLocation()
  if (loading) return null
  if (!person) return <Navigate to="/entrar" replace state={{ from: location.pathname + location.search }} />
  return <Layout />
}

function CreatorsOnly({ children }: { children: React.ReactNode }) {
  const { person } = useAuth()
  if (person?.role === "usuario") return <Navigate to="/" replace />
  return children
}

function Routed() {
  const { loading } = useAuth()
  if (loading) return null
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
        <Route path="pessoas" element={<PeoplePage />} />
        <Route path="meu-computador" element={<ComputerPage />} />
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
