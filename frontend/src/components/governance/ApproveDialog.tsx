import { useEffect, useState } from "react"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { NativeSelect } from "@/components/bits"
import { api, type ApprovalAudience, type ApprovalRequest, type Area, type PersonRow } from "@/lib/api"

/** The leader of a new item chooses who uses it before it goes live (ADR 046). */
export const needsAudience = (r: ApprovalRequest) => r.reason === "new" && r.my_steps.includes("leader")

export function ApproveDialog({ request, open, onOpenChange, onDone }: {
  request: ApprovalRequest; open: boolean; onOpenChange: (v: boolean) => void; onDone: (waiting: string) => void
}) {
  const ask = needsAudience(request)
  const [mode, setMode] = useState<ApprovalAudience["mode"] | null>(null)
  const [areaIds, setAreaIds] = useState<number[]>([])
  const [personIds, setPersonIds] = useState<number[]>([])
  const [areas, setAreas] = useState<Area[]>([])
  const [people, setPeople] = useState<PersonRow[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!open) return
    setMode(null)
    setAreaIds([])
    setPersonIds([])
    setError("")
    if (!ask) return
    api.areas().then(setAreas, () => {})
    api.people().then(setPeople, () => {})
  }, [open, ask])

  const others = areas.filter((a) => a.id !== request.area_id)
  const chosen = people.filter((p) => personIds.includes(p.id))
  const ready = !ask || (mode !== null && (mode !== "custom" || areaIds.length + personIds.length > 0))

  const confirm = async () => {
    setBusy(true)
    setError("")
    try {
      const audience = ask && mode
        ? { mode, area_ids: mode === "custom" ? areaIds : [], person_ids: mode === "custom" ? personIds : [] }
        : undefined
      const res = await api.approveRequest(request.id, audience)
      onOpenChange(false)
      onDone(res.waiting ?? "")
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível aprovar.")
    } finally {
      setBusy(false)
    }
  }

  const radio = (value: ApprovalAudience["mode"], label: string) => (
    <label className="flex items-center gap-2">
      <input type="radio" name={`audience-${request.id}`} className="accent-foreground" checked={mode === value} onChange={() => setMode(value)} />
      {label}
    </label>
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Aprovar {request.item_name}</DialogTitle>
          <DialogDescription>
            {ask ? "Antes de liberar, escolha quem vai usar." : "Fica registrado que você aprovou."}
          </DialogDescription>
        </DialogHeader>
        {ask && (
          <fieldset className="flex flex-col gap-2 text-sm">
            <legend className="mb-2 font-medium">Quem usa</legend>
            {radio("area", `Só ${request.area}`)}
            {radio("custom", `${request.area} e mais…`)}
            {mode === "custom" && (
              <div className="flex flex-col gap-3 pl-6">
                <div className="flex flex-col gap-1.5">
                  {others.map((a) => (
                    <label key={a.id} className="flex items-center gap-2">
                      <input type="checkbox" className="accent-foreground" checked={areaIds.includes(a.id)}
                        onChange={(e) => setAreaIds(e.target.checked ? [...areaIds, a.id] : areaIds.filter((x) => x !== a.id))} />
                      {a.name}
                    </label>
                  ))}
                </div>
                {chosen.length > 0 && (
                  <ul className="flex flex-col gap-1">
                    {chosen.map((p) => (
                      <li key={p.id} className="flex items-center gap-2">
                        {p.name}
                        <Button variant="ghost" size="icon" className="size-7" aria-label={`Tirar ${p.name}`} onClick={() => setPersonIds(personIds.filter((x) => x !== p.id))}>
                          <X />
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
                <NativeSelect aria-label="Adicionar pessoa" className="max-w-xs" value={0}
                  onChange={(e) => { const id = Number(e.target.value); if (id) setPersonIds([...personIds, id]) }}>
                  <option value={0}>Adicionar pessoa…</option>
                  {people.filter((p) => !personIds.includes(p.id)).map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
                </NativeSelect>
              </div>
            )}
            {radio("company", "Empresa toda")}
          </fieldset>
        )}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button onClick={confirm} disabled={!ready || busy}>Aprovar e liberar</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
