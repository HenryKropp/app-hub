import { createContext, useContext, useEffect, useState, type ReactNode } from "react"

export type ThemePref = "light" | "dark" | "system"

type ThemeCtx = { pref: ThemePref; resolved: "light" | "dark"; setPref: (p: ThemePref) => void }

const Ctx = createContext<ThemeCtx>({ pref: "system", resolved: "light", setPref: () => {} })
const KEY = "apphub-theme"

function readPref(): ThemePref {
  try {
    const v = localStorage.getItem(KEY)
    if (v === "light" || v === "dark" || v === "system") return v
  } catch {
    /* storage unavailable */
  }
  return "light"
}

function systemDark() {
  return typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [pref, setPrefState] = useState<ThemePref>(readPref)
  const [sysDark, setSysDark] = useState<boolean>(!!systemDark())

  useEffect(() => {
    const mq = window.matchMedia?.("(prefers-color-scheme: dark)")
    if (!mq) return
    const onChange = () => setSysDark(mq.matches)
    mq.addEventListener("change", onChange)
    return () => mq.removeEventListener("change", onChange)
  }, [])

  const resolved = pref === "system" ? (sysDark ? "dark" : "light") : pref

  useEffect(() => {
    document.documentElement.classList.toggle("dark", resolved === "dark")
  }, [resolved])

  const setPref = (p: ThemePref) => {
    setPrefState(p)
    try {
      localStorage.setItem(KEY, p)
    } catch {
      /* storage unavailable */
    }
  }

  return <Ctx.Provider value={{ pref, resolved, setPref }}>{children}</Ctx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = () => useContext(Ctx)
