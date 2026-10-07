import { useCallback, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, type ConfigItem, type EnvVar } from "@/lib/api"
import { errText } from "./settings"

// Names the Hub gives the app itself; an app setting cannot take them.
const HUB_NAMES = ["PORT", "BASE_URL", "APPHUB_DATA_DIR", "DATABASE_URL"]

function ConfigRow({ appId, item, onChanged }: { appId: number; item: ConfigItem; onChanged: () => void }) {
  const [value, setValue] = useState(item.secret ? "" : item.value)
  const [replacing, setReplacing] = useState(false)
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState("")
  const [error, setError] = useState("")
  const id = `cfg-${item.name}`

  const save = async (v: string) => {
    setBusy(true)
    setError("")
    setMsg("")
    try {
      const r = await api.shared.setConfig(appId, item.name, v)
      setMsg(r.needs_restart ? "Salvo. Religue o app para valer." : "Salvo.")
      if (item.secret) {
        setValue("")
        setReplacing(false)
      }
      onChanged()
    } catch (e) {
      setError(errText(e, "Não consegui salvar."))
    } finally {
      setBusy(false)
    }
  }
  const clear = () => {
    if (!window.confirm(`Apagar ${item.label}? O app fica sem esse valor.`)) return
    void save("")
  }

  const secretSaved = item.secret && item.has_value && !replacing
  return (
    <li className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-baseline gap-x-2">
        {secretSaved ? (
          // No field to point at while the saved password is only "Guardada".
          <span className="text-sm leading-none font-medium">{item.label || item.name}</span>
        ) : (
          <Label htmlFor={id}>{item.label || item.name}</Label>
        )}
        <code className="font-mono text-xs text-muted-foreground">{item.name}</code>
        {item.required && <span className="text-xs text-muted-foreground">obrigatório</span>}
        {item.required && !item.has_value && <span className="text-xs text-warn">sem valor</span>}
      </div>
      {secretSaved ? (
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-muted-foreground">Guardada</span>
          <Button size="sm" variant="outline" aria-label={`Trocar ${item.label}`} onClick={() => setReplacing(true)}>Trocar</Button>
          <Button size="sm" variant="ghost" aria-label={`Apagar ${item.label}`} disabled={busy} onClick={clear}>Apagar</Button>
        </div>
      ) : (
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input
            id={id}
            type={item.secret ? "password" : "text"}
            autoComplete={item.secret ? "new-password" : "off"}
            value={value}
            placeholder={item.default ? `Padrão: ${item.default}` : undefined}
            onChange={(e) => setValue(e.target.value)}
            className="sm:max-w-md"
          />
          <div className="flex gap-2">
            <Button size="sm" variant="outline" className="h-8" aria-label={`Salvar ${item.label}`} disabled={busy || (item.secret && !value)} onClick={() => save(value)}>Salvar</Button>
            {replacing && <Button size="sm" variant="ghost" className="h-8" onClick={() => { setReplacing(false); setValue("") }}>Cancelar</Button>}
          </div>
        </div>
      )}
      {msg && <p className="text-xs text-ok" role="status">{msg}</p>}
      {error && <p className="text-xs text-destructive" role="alert">{error}</p>}
    </li>
  )
}

function AddSetting({ onAdd, onDone }: { onAdd: (e: EnvVar) => Promise<void>; onDone: () => void }) {
  const [name, setName] = useState("")
  const [label, setLabel] = useState("")
  const [secret, setSecret] = useState(false)
  const [required, setRequired] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const add = async () => {
    const n = name.trim()
    if (!/^[A-Z][A-Z0-9_]*$/.test(n)) {
      setError("Use só letras maiúsculas, números e _ no nome, começando por letra (ex.: SMTP_PASSWORD).")
      return
    }
    if (HUB_NAMES.includes(n)) {
      setError(`${n} é dado pelo Hub. Escolha outro nome.`)
      return
    }
    setBusy(true)
    setError("")
    try {
      await onAdd({ name: n, label: label.trim() || n, secret, required })
      onDone()
    } catch (e) {
      setError(errText(e, "Não consegui adicionar."))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-xs" role="group" aria-label="Nova configuração">
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="cfg-new-name">Nome no código</Label>
          <Input id="cfg-new-name" value={name} placeholder="SMTP_PASSWORD" onChange={(e) => setName(e.target.value.toUpperCase())} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="cfg-new-label">Nome para as pessoas</Label>
          <Input id="cfg-new-label" value={label} placeholder="Senha do e-mail" onChange={(e) => setLabel(e.target.value)} />
        </div>
      </div>
      <div className="flex flex-wrap gap-x-5 gap-y-2 text-sm">
        <label className="inline-flex items-center gap-2">
          <Checkbox checked={secret} onCheckedChange={(c) => setSecret(c === true)} /> É uma senha
        </label>
        <label className="inline-flex items-center gap-2">
          <Checkbox checked={required} onCheckedChange={(c) => setRequired(c === true)} /> Obrigatória
        </label>
      </div>
      {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
      <div className="flex gap-2">
        <Button size="sm" onClick={add} disabled={busy}>Adicionar</Button>
        <Button size="sm" variant="ghost" onClick={onDone} disabled={busy}>Cancelar</Button>
      </div>
    </div>
  )
}

/** The settings and passwords the app reads. Saved passwords never come back: only "Guardada", Trocar and Apagar. */
export function SharedSettingsForm({ appId, onAdd }: { appId: number; onAdd?: (e: EnvVar) => Promise<void> }) {
  const [items, setItems] = useState<ConfigItem[] | null>(null)
  const [error, setError] = useState("")
  const [adding, setAdding] = useState(false)

  const load = useCallback(() => {
    api.shared.config(appId).then(
      (c) => {
        setItems(c)
        setError("")
      },
      (e) => setError(errText(e, "Não consegui carregar as configurações.")),
    )
  }, [appId])
  useEffect(load, [load])

  if (error && !items) return <p className="text-sm text-destructive" role="alert">{error}</p>
  if (!items) return <p className="text-sm text-muted-foreground">Carregando…</p>
  return (
    <div className="flex flex-col gap-4">
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Este app não pede configurações.</p>
      ) : (
        <ul className="flex flex-col gap-4">
          {items.map((i) => <ConfigRow key={i.name} appId={appId} item={i} onChanged={load} />)}
        </ul>
      )}
      {onAdd && (adding ? (
        <AddSetting onAdd={onAdd} onDone={() => { setAdding(false); load() }} />
      ) : (
        <div>
          <Button size="sm" variant="outline" onClick={() => setAdding(true)}>Adicionar configuração</Button>
        </div>
      ))}
    </div>
  )
}
