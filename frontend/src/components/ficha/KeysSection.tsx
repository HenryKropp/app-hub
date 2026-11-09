import { useCallback, useEffect, useRef, useState } from "react"
import { KeyRound } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, type KeyItem, type KeysList } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { Section } from "./Part"
import { errText } from "./shared/settings"

/** The script's vault (cofre): names and state for everyone who sees the
 * app; fill, show, replace and delete for the owner or an administrator.
 * A shown value lives only in this component's memory until "Esconder". */
function KeyRow({ appId, item, canManage, onChanged }: { appId: number; item: KeyItem; canManage: boolean; onChanged: () => void }) {
  const [value, setValue] = useState("")
  const [replacing, setReplacing] = useState(false)
  const [shown, setShown] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const label = item.label || item.name
  const id = `key-${item.name}`

  const act = async (fn: () => Promise<void>, fallback: string) => {
    setBusy(true)
    setError("")
    try {
      await fn()
    } catch (e) {
      setError(errText(e, fallback))
    } finally {
      setBusy(false)
    }
  }
  const save = () => act(async () => {
    await api.keys.set(appId, item.name, value)
    setValue("")
    setReplacing(false)
    setShown(null)
    onChanged()
  }, "Não consegui guardar a chave.")
  const remove = () => {
    if (!window.confirm(`Apagar ${label} do cofre? O script fica sem esse valor.`)) return
    void act(async () => {
      await api.keys.remove(appId, item.name)
      setShown(null)
      onChanged()
    }, "Não consegui apagar a chave.")
  }
  const reveal = () => act(async () => setShown((await api.keys.reveal(appId, item.name)).value), "Não consegui mostrar a chave.")

  const editing = canManage && (!item.has_value || replacing)
  return (
    <li className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-baseline gap-x-2">
        {editing ? <Label htmlFor={id}>{label}</Label> : <span className="text-sm leading-none font-medium">{label}</span>}
        <code className="font-mono text-xs text-muted-foreground">{item.name}</code>
        {item.required && <span className="text-xs text-muted-foreground">obrigatória</span>}
      </div>
      <p className="text-xs text-muted-foreground">
        {item.has_value
          ? `Preenchida${item.updated_by ? ` por ${item.updated_by}` : ""}${item.updated_at ? ` ${formatWhen(item.updated_at)}` : ""}`
          : <span className={item.required ? "text-warn" : undefined}>Vazia</span>}
      </p>
      {shown !== null && (
        <code className="w-fit max-w-full break-all rounded-sm bg-secondary px-2 py-1 font-mono text-xs">{shown}</code>
      )}
      {editing ? (
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input id={id} type="password" autoComplete="new-password" value={value} onChange={(e) => setValue(e.target.value)} className="sm:max-w-md" />
          <div className="flex gap-2">
            <Button size="sm" variant="outline" className="h-8" aria-label={`Salvar ${label}`} disabled={busy || !value} onClick={save}>Salvar</Button>
            {replacing && <Button size="sm" variant="ghost" className="h-8" onClick={() => { setReplacing(false); setValue("") }}>Cancelar</Button>}
          </div>
        </div>
      ) : canManage && item.has_value && (
        <div className="flex flex-wrap gap-2">
          {shown === null
            ? <Button size="sm" variant="outline" aria-label={`Mostrar ${label}`} disabled={busy} onClick={reveal}>Mostrar</Button>
            : <Button size="sm" variant="outline" aria-label={`Esconder ${label}`} onClick={() => setShown(null)}>Esconder</Button>}
          <Button size="sm" variant="outline" aria-label={`Trocar ${label}`} onClick={() => { setReplacing(true); setShown(null) }}>Trocar</Button>
          <Button size="sm" variant="ghost" aria-label={`Apagar ${label}`} disabled={busy} onClick={remove}>Apagar</Button>
        </div>
      )}
      {error && <p className="text-xs text-destructive" role="alert">{error}</p>}
    </li>
  )
}

export function KeysSection({ appId }: { appId: number }) {
  const [list, setList] = useState<KeysList | null>(null)
  const [error, setError] = useState("")
  const ref = useRef<HTMLDivElement>(null)
  const load = useCallback(() => {
    api.keys.list(appId).then(setList, (e) => setError(errText(e, "Não consegui abrir o cofre.")))
  }, [appId])
  useEffect(load, [load])
  // The link Claude gives (…/apps/N#chaves) opens straight on the vault.
  useEffect(() => {
    if (list && window.location.hash === "#chaves") ref.current?.scrollIntoView({ block: "start" })
  }, [list])

  if (!list && !error) return null
  const labelOf = (name: string) => list?.keys.find((k) => k.name === name)?.label || name
  return (
    <div ref={ref} id="chaves" className="scroll-mt-4">
      <Section title="Cofre" aside={<p className="text-xs text-muted-foreground">As senhas que o script usa. O valor só vai para o script na hora de rodar.</p>}>
        {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
        {list && (
          <div className="flex flex-col gap-4">
            {list.locked && (
              <p className="text-sm text-destructive">O cofre está fechado neste servidor. Fale com o responsável pelo Ziv.</p>
            )}
            {list.missing.length > 0 && (
              <p className="flex gap-2 text-sm">
                <KeyRound className="mt-0.5 size-4 shrink-0 text-warn" />
                O script não roda até preencher: {list.missing.map(labelOf).join(", ")}.
              </p>
            )}
            <ul className="flex flex-col gap-4">
              {list.keys.map((k) => <KeyRow key={k.name} appId={appId} item={k} canManage={list.can_manage && !list.locked} onChanged={load} />)}
            </ul>
            {!list.can_manage && (
              <p className="text-xs text-muted-foreground">Só o dono do app ou um administrador mexe no cofre.</p>
            )}
          </div>
        )}
      </Section>
    </div>
  )
}
