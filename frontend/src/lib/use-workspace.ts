import { useCallback, useEffect, useState } from "react"
import { api, type Workspace } from "@/lib/api"

export function useWorkspace() {
  const [workspace, setWorkspace] = useState<Workspace | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    api.workspace().then((value) => {
      if (active) setWorkspace(value)
    }, (reason: Error) => {
      if (active) {
        setWorkspace(null)
        setError(reason.message)
      }
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [attempt])

  const retry = useCallback(() => {
    setLoading(true)
    setError("")
    setAttempt((value) => value + 1)
  }, [])

  return { workspace, loading, error, retry }
}
