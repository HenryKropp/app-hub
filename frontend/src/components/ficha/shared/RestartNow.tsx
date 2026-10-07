import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { api } from "@/lib/api"
import { errText } from "./settings"

/** "Religar agora": after a change that only counts once the running app starts again. */
export function RestartNow({ appId, onDone }: { appId: number; onDone?: () => void }) {
  const [busy, setBusy] = useState(false)
  const restart = async () => {
    setBusy(true)
    try {
      await api.shared.start(appId)
      toast.success("Religando o app. Em alguns segundos ele volta com a mudança.")
      onDone?.()
    } catch (e) {
      toast.error(errText(e, "Não consegui religar o app. Tente de novo em alguns segundos."))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Button size="sm" variant="outline" onClick={restart} disabled={busy}>{busy ? "Religando…" : "Religar agora"}</Button>
  )
}
