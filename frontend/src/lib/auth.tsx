import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react"
import { api, type Person } from "./api"

type AuthCtx = {
  loading: boolean
  needsSetup: boolean
  person: Person | null
  refresh: () => Promise<void>
  setPerson: (p: Person | null) => void
}

const Ctx = createContext<AuthCtx>({
  loading: true,
  needsSetup: false,
  person: null,
  refresh: async () => {},
  setPerson: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [loading, setLoading] = useState(true)
  const [needsSetup, setNeedsSetup] = useState(false)
  const [person, setPerson] = useState<Person | null>(null)

  const refresh = useCallback(async () => {
    try {
      const s = await api.status()
      setNeedsSetup(s.needs_setup)
      setPerson(s.person)
    } catch {
      setPerson(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const set = (p: Person | null) => {
    setPerson(p)
    if (p) setNeedsSetup(false)
  }

  return <Ctx.Provider value={{ loading, needsSetup, person, refresh, setPerson: set }}>{children}</Ctx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useAuth = () => useContext(Ctx)
