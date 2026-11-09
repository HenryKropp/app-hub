import { useCallback, useEffect, useRef, useState } from "react"
import { KeyRound } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, type SettingItem, type SettingsList } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { Section } from "./Part"
import { RestartNow } from "./shared/RestartNow"
import { errText } from "./shared/settings"

/** Kinds of app with Configurações: every app the Agente opens on a computer. */
export const VAULT_KINDS = ["script", "web", "desktop", "scheduled", "executable"]

/** The app's Configurações (ADR 032): the values it reads from its
 * environment, in two blocks. "Chaves e senhas" stay hidden: a shown value
 * lives only in this component's memory until "Esconder". "Outros ajustes"
 * (a folder, an e-mail) show as they are. Names and state for everyone who
 * sees the app; changes for the owner or an administrator. One value serves
 * the computer and the app's 24h version. */
type Changed = (needsRestart?: boolean) => void

const labelOf = (item: SettingItem) => item.label || item.name

function Header({ item, field }: { item: SettingItem; field: string | null }) {
  const label = labelOf(item)
  return (
    <div className="flex flex-wrap items-baseline gap-x-2">
      {field ? <Label htmlFor={field}>{label}</Label> : <span className={`text-sm leading-none font-medium${label === item.name ? " font-mono" : ""}`}>{label}</span>}
      {item.label && item.label !== item.name && <code className="font-mono text-xs text-muted-foreground">{item.name}</code>}
      {item.required && <span className="text-xs text-muted-foreground">obrigatório</span>}
    </div>
  )
}

function State({ item }: { item: SettingItem }) {
  return (
    <p className="text-xs text-muted-foreground">
      {item.has_value
        ? `Preenchido${item.updated_by ? ` por ${item.updated_by}` : ""}${item.updated_at ? ` ${formatWhen(item.updated_at)}` : ""}`
        : <span className={item.required ? "text-warn" : undefined}>{item.default ? `Vazio (usa o padrão: ${item.default})` : "Vazio"}</span>}
    </p>
  )
}

/** Runs one change with busy and error state. */
function useAct() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
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
  return { busy, error, act }
}

function SecretRow({ appId, item, canManage, onChanged }: { appId: number; item: SettingItem; canManage: boolean; onChanged: Changed }) {
  const [value, setValue] = useState("")
  const [replacing, setReplacing] = useState(false)
  const [shown, setShown] = useState<string | null>(null)
  const { busy, error, act } = useAct()
  const label = labelOf(item)
  const id = `key-${item.name}`

  const save = () => act(async () => {
    const r = await api.keys.set(appId, item.name, value)
    setValue("")
    setReplacing(false)
    setShown(null)
    onChanged(r.needs_restart)
  }, "Não consegui guardar a senha.")
  const remove = () => {
    if (!window.confirm(`Apagar ${label}? O app fica sem esse valor.`)) return
    void act(async () => {
      const r = await api.keys.remove(appId, item.name)
      setShown(null)
      onChanged(r.needs_restart)
    }, "Não consegui apagar a senha.")
  }
  const reveal = () => act(async () => setShown((await api.keys.reveal(appId, item.name)).value), "Não consegui mostrar a senha.")

  const editing = canManage && (!item.has_value || replacing)
  return (
    <li className="flex flex-col gap-1.5">
      <Header item={item} field={editing ? id : null} />
      <State item={item} />
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

function PlainRow({ appId, item, canManage, onChanged }: { appId: number; item: SettingItem; canManage: boolean; onChanged: Changed }) {
  const [value, setValue] = useState(item.value ?? "")
  const { busy, error, act } = useAct()
  const label = labelOf(item)
  const id = `cfg-${item.name}`
  // A new answer from the server (after a save) is the value on screen.
  useEffect(() => setValue(item.value ?? ""), [item.value])

  const save = () => act(async () => {
    const r = await api.settings.set(appId, item.name, value.trim())
    onChanged(r.needs_restart)
  }, "Não consegui salvar.")
  const remove = () => {
    if (!window.confirm(`Apagar ${label}? O app fica sem esse valor.`)) return
    void act(async () => {
      const r = await api.settings.remove(appId, item.name)
      onChanged(r.needs_restart)
    }, "Não consegui apagar.")
  }

  return (
    <li className="flex flex-col gap-1.5">
      <Header item={item} field={canManage ? id : null} />
      <State item={item} />
      {canManage && (
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input id={id} value={value} autoComplete="off" placeholder={item.default ? `Padrão: ${item.default}` : undefined}
            onChange={(e) => setValue(e.target.value)} className="sm:max-w-md" />
          <div className="flex gap-2">
            <Button size="sm" variant="outline" className="h-8" aria-label={`Salvar ${label}`} disabled={busy || !value.trim() || value === (item.value ?? "")} onClick={save}>Salvar</Button>
            {item.has_value && <Button size="sm" variant="ghost" className="h-8" aria-label={`Apagar ${label}`} disabled={busy} onClick={remove}>Apagar</Button>}
          </div>
        </div>
      )}
      {error && <p className="text-xs text-destructive" role="alert">{error}</p>}
    </li>
  )
}

/** Normalizes what a person types into an environment name: "api token" → "API_TOKEN". */
export function keyName(typed: string) {
  return typed.trim().toUpperCase().normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/[^A-Z0-9]+/g, "_").replace(/^_+|_+$/g, "")
}

/** "Adicionar valor": a new value, secret or not, for the owner or an administrator. */
function AddValue({ appId, locked, what, onAdded }: { appId: number; locked: boolean; what: string; onAdded: Changed }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [value, setValue] = useState("")
  // A password by default: forgetting the box never leaves a secret in sight.
  const [secret, setSecret] = useState(!locked)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const env = keyName(name)
  const close = () => { setOpen(false); setName(""); setValue(""); setSecret(!locked); setError("") }
  const save = async () => {
    setBusy(true)
    setError("")
    try {
      const r = secret ? await api.keys.set(appId, env, value) : await api.settings.set(appId, env, value.trim())
      close()
      onAdded(r.needs_restart)
    } catch (e) {
      setError(errText(e, "Não consegui guardar."))
    } finally {
      setBusy(false)
    }
  }
  if (!open) {
    return <Button size="sm" variant="outline" className="w-fit" onClick={() => setOpen(true)}>Adicionar valor</Button>
  }
  return (
    <div className="flex flex-col gap-3 rounded-md border p-3" role="group" aria-label="Novo valor">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="new-setting-name">Nome</Label>
        <Input id="new-setting-name" value={name} autoComplete="off" placeholder="PASTA_RELATORIOS" onChange={(e) => setName(e.target.value)} className="font-mono sm:max-w-md" />
        <p className="text-xs text-muted-foreground">
          {env && env !== name.trim()
            ? <>Vai ficar <code className="font-mono">{env}</code>. </>
            : null}
          {what === "o script" ? "O script" : "O app"} lê o valor por esse nome: <code className="font-mono">os.environ["{env || "NOME"}"]</code>
        </p>
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="new-setting-value">Valor</Label>
        <Input id="new-setting-value" type={secret ? "password" : "text"} autoComplete={secret ? "new-password" : "off"} value={value} onChange={(e) => setValue(e.target.value)} className="sm:max-w-md" />
      </div>
      <label className="inline-flex items-center gap-2 text-sm">
        <Checkbox checked={secret} disabled={locked} onCheckedChange={(c) => setSecret(c === true)} /> É uma senha (fica escondida)
      </label>
      {error && <p className="text-xs text-destructive" role="alert">{error}</p>}
      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={busy || !env || !value.trim()} onClick={save}>Salvar</Button>
        <Button size="sm" variant="ghost" onClick={close}>Cancelar</Button>
      </div>
    </div>
  )
}

export function SettingsSection({ appId, kind = "script", onChanged }: { appId: number; kind?: string; onChanged?: () => void }) {
  const [list, setList] = useState<SettingsList | null>(null)
  const [error, setError] = useState("")
  const [restart, setRestart] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const load = useCallback(() => {
    api.settings.list(appId).then(setList, (e) => setError(errText(e, "Não consegui abrir as configurações.")))
  }, [appId])
  useEffect(load, [load])
  const changed: Changed = (needsRestart) => {
    if (needsRestart) setRestart(true)
    load()
    onChanged?.()
  }
  // The link Claude gives (…/apps/N#chaves) opens straight here.
  useEffect(() => {
    if (list && window.location.hash === "#chaves") ref.current?.scrollIntoView({ block: "start" })
  }, [list])

  if (!list && !error) return null
  const script = kind === "script"
  const what = script ? "o script" : "o app"
  const secrets = list?.items.filter((i) => i.secret) ?? []
  const plain = list?.items.filter((i) => !i.secret) ?? []
  const manage = !!list?.can_manage
  return (
    <div ref={ref} id="chaves" className="scroll-mt-4">
      {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
      {list && (
        <>
          <div className="flex flex-col gap-3 pb-6">
            <p className="max-w-[68ch] text-sm text-muted-foreground">
              {script
                ? "Os valores que o script usa, no lugar de deixá-los no código ou no .env. Ele recebe cada um na hora de rodar."
                : "Os valores que o app usa, no lugar do .env. Os mesmos valores valem quando ele abre no seu computador e na versão ligada no servidor."}
            </p>
            {list.locked && (
              <p className="text-sm text-destructive">As senhas estão trancadas neste servidor. Fale com o responsável pelo Ziv.</p>
            )}
            {list.missing.length > 0 && (
              <p className="flex gap-2 text-sm">
                <KeyRound className="mt-0.5 size-4 shrink-0 text-warn" />
                Falta preencher: {list.missing.join(", ")}. Sem isso, {what} não funciona direito.
              </p>
            )}
          </div>

          <Section title="Chaves e senhas" aside={<p className="text-xs text-muted-foreground">Ficam escondidas. Quem cuida do app vê clicando em Mostrar.</p>}>
            {secrets.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nenhuma senha ainda. Guarde aqui as senhas e tokens que {what} usa. Pelo Claude, peça "guarde as senhas do .env no Ziv" e ele manda o arquivo direto, sem ver as senhas.</p>
            ) : (
              <ul className="flex flex-col gap-4">
                {secrets.map((k) => <SecretRow key={k.name} appId={appId} item={k} canManage={manage && !list.locked} onChanged={changed} />)}
              </ul>
            )}
          </Section>

          <Section title="Outros ajustes" aside={<p className="text-xs text-muted-foreground">Valores comuns, como uma pasta ou um e-mail. Aparecem como estão.</p>}>
            {plain.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nenhum ajuste ainda.</p>
            ) : (
              <ul className="flex flex-col gap-4">
                {plain.map((k) => <PlainRow key={k.name} appId={appId} item={k} canManage={manage} onChanged={changed} />)}
              </ul>
            )}
          </Section>

          <div className="flex flex-col gap-3 border-t border-border pt-6">
            {restart && (
              <p className="flex flex-wrap items-center gap-2 text-sm">
                A versão ligada no servidor só usa a mudança depois de religar. <RestartNow appId={appId} onDone={() => setRestart(false)} />
              </p>
            )}
            {manage
              ? <AddValue appId={appId} locked={list.locked} what={what} onAdded={changed} />
              : <p className="text-xs text-muted-foreground">Só o dono do app ou um administrador muda as configurações.</p>}
          </div>
        </>
      )}
    </div>
  )
}
