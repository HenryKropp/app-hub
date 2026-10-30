import { useCallback, useEffect, useState, type FormEvent } from "react"
import { AlertTriangle, Check, KeyRound, RefreshCw, RotateCw, Shield, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, type CredentialEvent, type CredentialInput, type CredentialMeta, type RoleInfo, type SecurityApp } from "@/lib/api"

type Props = { apps: SecurityApp[] }
type Environment = CredentialInput["environment"]
const environments: { value: Environment; label: string }[] = [
  { value: "development", label: "Desenvolvimento" },
  { value: "staging", label: "Homologação" },
  { value: "production", label: "Produção" },
]
const endpointHint = "HTTPS fixo, sem parâmetros de consulta ou informações de usuário."
const panelClass = "rounded-xl border border-border bg-card p-5 sm:p-6"
const fieldClass = "mt-1 block w-full rounded-md border border-input bg-background px-3 py-2 text-sm"

function safeError(error: unknown, secret = "") {
  const message = error instanceof Error ? error.message : "Não foi possível concluir a operação."
  return secret ? message.split(secret).join("[redigido]") : message
}

function validateEndpoint(raw: string) {
  try {
    const url = new URL(raw)
    return url.protocol === "https:" && !url.username && !url.password && !url.search && !url.hash
  } catch {
    return false
  }
}

export function CredentialVault({ apps }: Props) {
  const [credentials, setCredentials] = useState<CredentialMeta[]>([])
  const [roles, setRoles] = useState<RoleInfo[]>([])
  const [events, setEvents] = useState<CredentialEvent[]>([])
  const [available, setAvailable] = useState(false)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState("")
  const [actionError, setActionError] = useState("")
  const [notice, setNotice] = useState("")
  const [busy, setBusy] = useState(false)
  const [rewrapping, setRewrapping] = useState(false)
  const [checking, setChecking] = useState<string | null>(null)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [editing, setEditing] = useState<CredentialMeta | null>(null)
  const [appId, setAppId] = useState("")
  const [environment, setEnvironment] = useState<Environment>("development")
  const [roleId, setRoleId] = useState("")
  const [endpoint, setEndpoint] = useState("")
  const [value, setValue] = useState("")
  const [formError, setFormError] = useState("")
  const activeApps = apps.filter((app) => !app.suspended)

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError("")
    try {
      const [vault, roleList, audit] = await Promise.all([api.credentials(), api.roles(), api.credentialAudit()])
      setCredentials(vault.credentials)
      setAvailable(vault.available)
      setRoles(roleList)
      setEvents(audit)
    } catch (error) {
      setLoadError(safeError(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const clearForm = () => {
    setEditing(null)
    setAppId("")
    setEnvironment("development")
    setRoleId("")
    setEndpoint("")
    setValue("")
    setFormError("")
  }

  const startRotate = (credential: CredentialMeta) => {
    setEditing(credential)
    setAppId(String(credential.app_id))
    setEnvironment(credential.environment)
    setRoleId(String(credential.role_id))
    setEndpoint("")
    setValue("")
    setFormError("")
    const form = document.getElementById("credential-form")
    if (form && typeof form.scrollIntoView === "function") form.scrollIntoView({ behavior: "smooth", block: "start" })
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const submittedValue = value
    setFormError("")
    setActionError("")
    if (!available) {
      setFormError("O cofre está bloqueado. Configure o keyring antes de gravar ou rotacionar credenciais.")
      setValue("")
      return
    }
    if (!appId || !roleId || !submittedValue) {
      setFormError("Preencha aplicativo, ambiente, papel, endpoint e credencial.")
      setValue("")
      return
    }
    if (!validateEndpoint(endpoint)) {
      setFormError("Informe um endpoint HTTPS válido, sem usuário, senha, consulta ou fragmento.")
      setValue("")
      return
    }
    const input: CredentialInput = {
      app_id: Number(appId),
      environment,
      role_id: Number(roleId),
      endpoint: endpoint.trim(),
      value: submittedValue,
    }
    setBusy(true)
    try {
      if (editing) await api.replaceCredential(editing.id, input)
      else await api.createCredential(input)
      setNotice(editing ? "Credencial rotacionada. O aplicativo e o ambiente permaneceram inalterados." : "Credencial armazenada.")
      clearForm()
      await load()
    } catch (error) {
      setFormError(safeError(error, submittedValue))
      setValue("")
    } finally {
      setBusy(false)
      setValue("")
    }
  }

  const revoke = async (credential: CredentialMeta) => {
    if (!window.confirm(`Revogar credencial do aplicativo ${appName(apps, credential.app_id)} (${labelEnvironment(credential.environment)})?`)) return
    setRevoking(credential.id)
    setActionError("")
    setNotice("")
    try {
      await api.revokeCredential(credential.id)
      setNotice("Credencial revogada.")
      await load()
    } catch (error) {
      setActionError(safeError(error))
    } finally {
      setRevoking(null)
    }
  }

  const rewrap = async () => {
    setRewrapping(true)
    setActionError("")
    setNotice("")
    try {
      const result = await api.rewrapCredentials()
      setNotice(`${result.rewrapped} credencial(is) reprotegida(s) com a chave ativa. O token armazenado não foi alterado.`)
      await load()
    } catch (error) {
      setActionError(safeError(error))
    } finally {
      setRewrapping(false)
    }
  }

  const check = async (credential: CredentialMeta) => {
    const app = apps.find((item) => item.id === credential.app_id)
    if (!app || !app.approved || app.suspended || !roles.some((role) => role.id === credential.role_id)) {
      setActionError("A verificação exige aplicativo visível e aprovado, além de um papel ainda disponível.")
      return
    }
    setChecking(credential.id)
    setActionError("")
    setNotice("")
    try {
      const result = await api.checkCredential(credential)
      setNotice(`Verificação HEAD concluída. Status HTTP recebido do destino: ${result.upstream_status}.`)
    } catch (error) {
      setActionError(safeError(error))
    } finally {
      setChecking(null)
    }
  }

  const roleName = (id: number) => roles.find((role) => role.id === id)?.name ?? `Papel ${id}`
  const appNameFor = (id: number) => appName(apps, id)

  return (
    <section className={`${panelClass} mb-10`} aria-labelledby="credential-vault-heading">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex gap-3">
          <div className="mt-0.5 rounded-lg bg-brand/10 p-2 text-brand"><KeyRound className="size-5" /></div>
          <div>
            <h2 id="credential-vault-heading" className="font-heading text-xl">Cofre de credenciais</h2>
            <p className="mt-1 max-w-3xl text-sm text-muted-foreground">Tokens ficam fora do código e são exibidos apenas como metadados. O valor não pode ser lido, copiado ou recuperado por esta tela.</p>
          </div>
        </div>
        <Button type="button" variant="outline" disabled={rewrapping || !available} onClick={() => void rewrap()}>
          <RotateCw className={`mr-2 size-4 ${rewrapping ? "animate-spin" : ""}`} />
          {rewrapping ? "Reprotegendo…" : "Reproteger com chave ativa"}
        </Button>
      </div>
      <p className="mt-3 text-xs text-muted-foreground">A reproteção usa a chave mestra ativa carregada do segredo do espaço; ela não altera o token armazenado.</p>

      {!loading && !loadError && !available && <div className="mt-5 flex gap-3 rounded-lg border border-warn/35 bg-warn-soft/50 p-4 text-sm">
        <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warn" />
        <p><strong>Cofre bloqueado.</strong> Configure <code className="rounded bg-background px-1 py-0.5 font-mono text-xs">CREDENTIAL_VAULT_KEYRING</code> no servidor para armazenar, rotacionar ou reproteger valores. Metadados e revogação continuam disponíveis.</p>
      </div>}

      {loadError && <div className="mt-5 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm">
        <p role="alert">{loadError}</p>
        <Button className="mt-3" variant="outline" size="sm" onClick={() => void load()}><RefreshCw className="mr-2 size-4" />Tentar novamente</Button>
      </div>}
      {loading && <div role="status" aria-label="Carregando cofre" className="mt-5 space-y-3"><div className="h-10 animate-pulse rounded bg-muted" /><div className="h-16 animate-pulse rounded bg-muted" /></div>}
      {!loading && !loadError && <>
        {actionError && <p role="alert" className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}
        {notice && <p role="status" className="mt-4 rounded-md bg-ok-soft p-3 text-sm">{notice}</p>}

        <div className="mt-6">
          <div className="flex flex-wrap items-end justify-between gap-2">
            <div><h3 className="font-heading text-lg">Credenciais cadastradas</h3><p className="mt-1 text-xs text-muted-foreground">Somente identificadores, vínculo, revisão e estado. Nenhum valor secreto é retornado.</p></div>
            <Button type="button" variant="ghost" size="sm" onClick={() => void load()}><RefreshCw className="mr-2 size-4" />Atualizar</Button>
          </div>
          {credentials.length === 0 ? <p className="mt-3 rounded-lg border border-dashed border-border p-5 text-sm text-muted-foreground">Nenhuma credencial registrada. Adicione um token apenas quando o keyring estiver disponível.</p> : <div className="mt-3 space-y-2">
            {credentials.map((credential) => {
              const app = apps.find((item) => item.id === credential.app_id)
              const canCheck = Boolean(app?.approved && !app.suspended && roles.some((role) => role.id === credential.role_id))
              return <article key={credential.id} className="flex flex-col gap-3 rounded-lg border border-border/80 bg-background/60 p-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2"><h4 className="font-medium">{appNameFor(credential.app_id)}</h4><span className={`rounded-full px-2 py-0.5 text-xs ${credential.revoked ? "bg-muted text-muted-foreground" : "bg-ok-soft text-foreground"}`}>{credential.revoked ? "Revogada" : "Ativa"}</span></div>
                  <p className="mt-1 text-sm text-muted-foreground">{labelEnvironment(credential.environment)} · {roleName(credential.role_id)} · revisão {credential.revision}</p>
                  <p className="mt-1 font-mono text-xs text-muted-foreground">ID {credential.id} · Atualizada {new Date(credential.updated_at).toLocaleString("pt-BR")}</p>
                </div>
                <div className="flex flex-wrap gap-2">
                  {!credential.revoked && <Button type="button" size="sm" variant="outline" disabled={!available || busy} onClick={() => startRotate(credential)}><RotateCw className="mr-2 size-3.5" />Rotacionar</Button>}
                  {!credential.revoked && <Button type="button" size="sm" variant="outline" disabled={!available || !canCheck || checking === credential.id} title={!canCheck ? "Verificação exige aplicativo aprovado e visível" : "Envia uma solicitação HEAD pelo servidor"} onClick={() => void check(credential)}>{checking === credential.id ? "Verificando…" : "Testar HEAD"}</Button>}
                  {!credential.revoked && <Button type="button" size="sm" variant="destructive" disabled={revoking === credential.id} onClick={() => void revoke(credential)}><Trash2 className="mr-2 size-3.5" />{revoking === credential.id ? "Revogando…" : "Revogar"}</Button>}
                </div>
              </article>
            })}
          </div>}
        </div>

        <form id="credential-form" noValidate onSubmit={(event) => void submit(event)} className="mt-7 border-t border-border pt-6">
          <h3 className="font-heading text-lg">{editing ? "Rotacionar credencial" : "Adicionar credencial"}</h3>
          {editing && <p className="mt-1 text-sm text-muted-foreground">Aplicativo e ambiente não podem ser alterados durante a rotação. Para outro vínculo, crie uma credencial separada.</p>}
          <div className="mt-4 grid gap-4 sm:grid-cols-2">
            <label className="text-sm font-medium">Aplicativo
              <select required value={appId} disabled={Boolean(editing)} onChange={(event) => setAppId(event.target.value)} className={fieldClass}>
                <option value="">Selecione…</option>{(editing ? apps.filter((app) => app.id === editing.app_id) : activeApps).map((app) => <option key={app.id} value={app.id}>{app.name}{app.approved ? "" : " · não aprovado"}</option>)}
              </select>
            </label>
            <label className="text-sm font-medium">Ambiente
              <select required value={environment} disabled={Boolean(editing)} onChange={(event) => setEnvironment(event.target.value as Environment)} className={fieldClass}>
                {environments.map((env) => <option key={env.value} value={env.value}>{env.label}</option>)}
              </select>
            </label>
            <label className="text-sm font-medium">Papel
              <select required value={roleId} onChange={(event) => setRoleId(event.target.value)} className={fieldClass}>
                <option value="">Selecione…</option>{roles.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}
              </select>
            </label>
            <label className="text-sm font-medium sm:col-span-2">Endpoint HTTPS
              <Input className="mt-1" type="url" aria-label="Endpoint HTTPS" value={endpoint} onChange={(event) => setEndpoint(event.target.value)} placeholder="https://api.exemplo.com/v1/status" required aria-describedby="credential-endpoint-hint" />
              <span id="credential-endpoint-hint" className="mt-1 block text-xs font-normal text-muted-foreground">{endpointHint} O teste envia somente HEAD pelo servidor.</span>
            </label>
            <label className="text-sm font-medium sm:col-span-2">Token bearer
              <Input className="mt-1" type="password" aria-label="Token bearer" autoComplete="new-password" value={value} onChange={(event) => setValue(event.target.value)} required disabled={!available} aria-describedby="credential-value-hint" />
              <span id="credential-value-hint" className="mt-1 block text-xs font-normal text-muted-foreground">Entrada somente para gravação. Será limpa após o envio, inclusive em caso de falha.</span>
            </label>
          </div>
          {roles.length === 0 && <p className="mt-3 text-sm text-muted-foreground">Nenhum papel disponível. Crie um papel antes de cadastrar uma credencial.</p>}
          {activeApps.length === 0 && !editing && <p className="mt-3 text-sm text-muted-foreground">Nenhum aplicativo disponível neste espaço.</p>}
          {formError && <p role="alert" className="mt-3 text-sm text-destructive">{formError}</p>}
          <div className="mt-4 flex flex-wrap gap-2">
            <Button type="submit" disabled={busy || !available || roles.length === 0 || (activeApps.length === 0 && !editing)}>{busy ? "Armazenando…" : editing ? "Salvar rotação" : "Armazenar credencial"}</Button>
            {editing && <Button type="button" variant="ghost" onClick={clearForm}>Cancelar rotação</Button>}
          </div>
        </form>

        <div className="mt-7 border-t border-border pt-6">
          <h3 className="flex items-center gap-2 font-heading text-lg"><Shield className="size-4 text-brand" />Auditoria do cofre</h3>
          <p className="mt-1 text-xs text-muted-foreground">Eventos e revisões, sem conteúdo dos tokens.</p>
          {events.length === 0 ? <p className="mt-3 text-sm text-muted-foreground">Nenhum evento de credencial disponível.</p> : <ol className="mt-3 max-h-64 divide-y overflow-auto">{events.map((event) => <li key={event.id} className="flex flex-wrap items-center justify-between gap-2 py-2 text-sm"><span className="font-medium">{event.action} · revisão {event.revision}</span><time className="text-xs text-muted-foreground">{new Date(event.created_at).toLocaleString("pt-BR")}</time><span className="w-full text-xs text-muted-foreground">Credencial {event.credential_id} · {event.actor_id === 0 ? "Sistema" : `Pessoa ${event.actor_id}`}</span></li>)}</ol>}
        </div>
      </>}
      {loadError && !loading && <p className="mt-4 text-xs text-muted-foreground">O conteúdo do cofre não pôde ser atualizado; tente novamente para ver o estado atual.</p>}
      {!loading && available && <p className="mt-4 flex items-start gap-2 text-xs text-muted-foreground"><Check className="mt-0.5 size-3.5 shrink-0 text-brand" />A disponibilidade do keyring não altera aprovação de aplicativos ou permissões de acesso.</p>}
    </section>
  )
}

function appName(apps: SecurityApp[], id: number) {
  return apps.find((app) => app.id === id)?.name ?? `Aplicativo ${id}`
}

function labelEnvironment(environment: Environment) {
  return environments.find((item) => item.value === environment)?.label ?? environment
}
