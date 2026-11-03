import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { api, type ComputersInfo } from "@/lib/api"

type ComputerStatus = {
  info: ComputersInfo | null
  // null while unknown (first load, or the check failed): never warn on a guess.
  connected: boolean | null
  refresh: () => void
}

const Ctx = createContext<ComputerStatus>({ info: null, connected: null, refresh: () => {} })

// ComputerStatusProvider knows whether this person's computer has the Agente
// connected, for the warning strip and the Rodar check.
export function ComputerStatusProvider({ children, every = 15000 }: { children: ReactNode; every?: number }) {
  const [info, setInfo] = useState<ComputersInfo | null>(null)
  const refresh = useCallback(() => {
    api.computers().then(setInfo, () => {})
  }, [])
  useEffect(() => {
    refresh()
    const t = setInterval(refresh, every)
    return () => clearInterval(t)
  }, [refresh, every])
  const value = useMemo(
    () => ({ info, connected: info ? info.computers.some((c) => c.connected) : null, refresh }),
    [info, refresh],
  )
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export const useComputerStatus = () => useContext(Ctx)
