import { useCallback, useEffect, useRef, useState } from "react"
import { KeyRound } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, type KeyItem, type KeysList } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { Section } from "./Part"
import { RestartNow } from "./shared/RestartNow"
import { errText } from "./shared/settings"

/** Kinds of app with a vault: every app the Agente opens on a computer. */
export const VAULT_KINDS = ["script", "web", "desktop", "scheduled", "executable"]

/** The app's vault (cofre): names and state for everyone who sees the
 * app; fill, show, replace and delete for the owner or an administrator.
 * A shown value lives only in this component's memory until "Esconder".
 * One value serves the computer and the app's 24h version. */
type Changed = (needsRestart?: boolean) => void

function KeyRow({ appId, item, canManage, onChanged }: { appId: number; item: KeyItem; canManage: boolean; onChanged: Changed }) {
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
    const r = await api.keys.set(appId, item.name, value)
    setValue("")
    setReplacing(false)
    setShown(null)
    onChanged(r.needs_restart)
  }, "Não consegui guardar a chave.")
  const remove = () => {
    if (!window.confirm(`Apagar ${label} do cofre? O app fica sem esse valor.`)) return
    void act(async () => {
      const r = await api.keys.remove(appId, item.name)
      setShown(null)
      onChanged(r.needs_restart)
    }, "Não consegui apagar a chave.")
  }
  const reveal = () => act(async () => setShown((await api.keys.reveal(appId, item.name)).value), "Não consegui mostrar a chave.")

  const editing = canManage && (!item.has_value || replacing)
  return (
    <li className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-baseline gap-x-2">
        {editing ? <Label htmlFor={id}>{label}</Label> : <span className={`text-sm leading-none font-medium${label === item.name ? " font-mono" : ""}`}>{label}</span>}
        {item.label && item.label !== item.name && <code className="font-mono text-xs text-muted-foreground">{item.name}</code>}
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

/** Normalizes what a person types into an environment name: "api token" → "API_TOKEN". */
export function keyName(typed: string) {
  return typed.trim().toUpperCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "").replace(/[^A-Z0-9]+/g, "_").replace(/^_+|_+$/g, "")
}

/** "Adicionar chave": a new key with its value, for the owner or an administrator. */
function AddKey({ appId, onAdded }: { appId: number; onAdded: Changed }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [value, setValue] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const env = keyName(name)
  const close = () => { setOpen(false); setName(""); setValue(""); setError("") }
  const save = async () => {
    setBusy(true)
    setError("")
    try {
      const r = await api.keys.set(appId, env, value)
      close()
      onAdded(r.needs_restart)
    } catch (e) {
      setError(errText(e, "Não consegui guardar a chave."))
    } finally {
      setBusy(false)
    }
  }
  if (!open) {
    return <Button size="sm" variant="outline" className="w-fit" onClick={() => setOpen(true)}>Adicionar chave</Button>
  }
  return (
    <div className="flex flex-col gap-3 rounded-md border p-3">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="new-key-name">Nome</Label>
        <Input id="new-key-name" value={name} autoComplete="off" placeholder="BTG_SENHA" onChange={(e) => setName(e.target.value)} className="font-mono sm:max-w-md" />
        <p className="text-xs text-muted-foreground">
          {env && env !== name.trim()
            ? <>Vai ficar <code className="font-mono">{env}</code>. </>
            : null}
          O app lê o valor por esse nome: <code className="font-mono">os.environ["{env || "NOME"}"]</code>
        </p>
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="new-key-value">Valor</Label>
        <Input id="new-key-value" type="password" autoComplete="new-password" value={value} onChange={(e) => setValue(e.target.value)} className="sm:max-w-md" />
      </div>
      {error && <p className="text-xs text-destructive" role="alert">{error}</p>}
      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={busy || !env || !value} onClick={save}>Salvar</Button>
        <Button size="sm" variant="ghost" onClick={close}>Cancelar</Button>
      </div>
    </div>
  )
}

export function KeysSection({ appId, kind = "script" }: { appId: number; kind?: string }) {
  const [list, setList] = useState<KeysList | null>(null)
  const [error, setError] = useState("")
  const [restart, setRestart] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const load = useCallback(() => {
    api.keys.list(appId).then(setList, (e) => setError(errText(e, "Não consegui abrir o cofre.")))
  }, [appId])
  useEffect(load, [load])
  const changed: Changed = (needsRestart) => {
    if (needsRestart) setRestart(true)
    load()
  }
  // The link Claude gives (…/apps/N#chaves) opens straight on the vault.
  useEffect(() => {
    if (list && window.location.hash === "#chaves") ref.current?.scrollIntoView({ block: "start" })
  }, [list])

  if (!list && !error) return null
  const labelOf = (name: string) => list?.keys.find((k) => k.name === name)?.label || name
  const script = kind === "script"
  const what = script ? "o script" : "o app"
  return (
    <div ref={ref} id="chaves" className="scroll-mt-4">
      <Section title="Cofre" aside={<p className="text-xs text-muted-foreground">{script
        ? "As senhas que o script usa. O valor só vai para o script na hora de rodar."
        : "As senhas que o app usa, no lugar do .env. Valem quando ele abre no seu computador e na versão ligada no servidor."}</p>}>
        {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
        {list && (
          <div className="flex flex-col gap-4">
            {list.locked && (
              <p className="text-sm text-destructive">O cofre está fechado neste servidor. Fale com o responsável pelo Ziv.</p>
            )}
            {list.missing.length > 0 && (
              <p className="flex gap-2 text-sm">
                <KeyRound className="mt-0.5 size-4 shrink-0 text-warn" />
                {script ? "O script não roda" : "O app não abre"} até preencher: {list.missing.map(labelOf).join(", ")}.
              </p>
            )}
            {list.keys.length === 0 && (
              <p className="text-sm text-muted-foreground">Nenhuma chave ainda. Guarde aqui as senhas e tokens que {what} usa, em vez de deixá-los no código ou no .env. Pelo Claude, peça "preencha o cofre com o .env" e ele manda o arquivo direto ao Ziv, sem ver as senhas.</p>
            )}
            {list.keys.length > 0 && (
              <ul className="flex flex-col gap-4">
                {list.keys.map((k) => <KeyRow key={k.name} appId={appId} item={k} canManage={list.can_manage && !list.locked} onChanged={changed} />)}
              </ul>
            )}
            {restart && (
              <p className="flex flex-wrap items-center gap-2 text-sm">
                A versão ligada no servidor só usa a mudança depois de religar. <RestartNow appId={appId} onDone={() => setRestart(false)} />
              </p>
            )}
            {list.can_manage && !list.locked && <AddKey appId={appId} onAdded={changed} />}
            {!list.can_manage && (
              <p className="text-xs text-muted-foreground">Só o dono do app ou um administrador mexe no cofre.</p>
            )}
          </div>
        )}
      </Section>
    </div>
  )
}
