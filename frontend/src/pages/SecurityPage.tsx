import { useCallback, useEffect, useState, type FormEvent } from "react"
import { AlertTriangle, Download, LockKeyhole, RefreshCw, ShieldCheck } from "lucide-react"
import { Field, PageHeader } from "@/components/bits"
import { CredentialVault } from "@/components/security/CredentialVault"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ApiError, api, type AuditInfo, type SecurityApp, type SecurityInfo, type SecuritySettings, type Workspace } from "@/lib/api"

const initialSettings: SecuritySettings = { retention_days: 30, require_app_approval: true, block_embedded_secrets: true, local_execution_enabled: true }
const inputClass = "mt-1 block w-full rounded-md border border-input bg-background px-3 py-2 text-sm"

export function SecurityPage() {
  const [info, setInfo] = useState<SecurityInfo | null>(null)
  const [audit, setAudit] = useState<AuditInfo | null>(null)
  const [workspace, setWorkspace] = useState<Workspace | null>(null)
  const [settings, setSettings] = useState<SecuritySettings>(initialSettings)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [denied, setDenied] = useState(false)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState("")
  const [purging, setPurging] = useState(false)
  const [companyName, setCompanyName] = useState("")
  const [companySlug, setCompanySlug] = useState("")
  const [password, setPassword] = useState("")
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState("")
  const [createBusy, setCreateBusy] = useState(false)
  const [exportBusy, setExportBusy] = useState(false)

  const load = useCallback(async () => {
    setLoading(true); setError(""); setNotice(""); setDenied(false)
    try {
      const [security, events, currentWorkspace] = await Promise.all([api.security(), api.securityAudit(), api.workspace()])
      setInfo(security); setSettings(security.settings); setAudit(events); setWorkspace(currentWorkspace)
    } catch (e) {
      if (e instanceof ApiError && e.status === 403) { setDenied(true); return }
      setError(e instanceof Error ? e.message : "Não foi possível carregar as informações de segurança.")
    } finally { setLoading(false) }
  }, [])
  useEffect(() => { void load() }, [load])

  const saveSettings = async (event: FormEvent) => {
    event.preventDefault()
    if (!settings.local_execution_enabled && !window.confirm("Confirme: novos pedidos serão bloqueados e jobs ainda não reivindicados serão cancelados. Scripts já em execução e agentes offline NÃO serão interrompidos. Deseja continuar?")) return
    setSaving(true); setNotice(""); setError("")
    try {
      const updated = await api.saveSecuritySettings(settings)
      setSettings(updated); setInfo((current) => current ? { ...current, settings: updated } : current)
      setNotice("Configurações salvas.")
    } catch (e) { setError(e instanceof Error ? e.message : "Não foi possível salvar as configurações.") }
    finally { setSaving(false) }
  }

  const saveApp = async (app: SecurityApp, form: HTMLFormElement) => {
    const data = new FormData(form)
    const suspended = data.get("suspended") === "on"
    if (suspended && !app.suspended && !window.confirm(`Confirme: suspender ${app.name} impedirá sua execução por toda a empresa.`)) {
      const control = form.elements.namedItem("suspended")
      if (control instanceof HTMLInputElement) control.checked = false
      return
    }
    setNotice(""); setError("")
    try {
      const updated = await api.saveSecurityApp(app.id, {
        classification: String(data.get("classification")) as "internal" | "confidential" | "restricted",
        purpose: String(data.get("purpose") ?? "").slice(0, 1000),
        external_domains: String(data.get("domains") ?? "").split(/\n|,/).map((domain) => domain.trim()).filter(Boolean),
        approved: data.get("approved") === "on", suspended,
      })
      setInfo((current) => current ? { ...current, apps: current.apps.map((item) => item.id === updated.id ? updated : item) } : current)
      setNotice(`Perfil de ${updated.name} salvo.`)
    } catch (e) { setError(e instanceof Error ? e.message : "Não foi possível salvar o perfil.") }
  }

  const purge = async () => {
    if (!window.confirm(`Excluir permanentemente metadados e payloads das execuções finalizadas além de ${settings.retention_days} dias? Esta ação não pode ser desfeita. Isso NÃO apaga arquivos Windows nem backups do banco de dados.`)) return
    setPurging(true); setError(""); setNotice("")
    try {
      const result = await api.purgeRuns()
      setNotice(`${result.deleted_runs} execuções finalizadas removidas conforme a retenção.`)
      const latest = await api.securityAudit(); setAudit(latest)
    } catch (e) { setError(e instanceof Error ? e.message : "Não foi possível aplicar a retenção.") }
    finally { setPurging(false) }
  }

  const createCompany = async (event: FormEvent) => {
    event.preventDefault(); setCreateBusy(true); setCreateError("")
    try {
      const result = await api.createWorkspace({ name: companyName.trim(), slug: companySlug.trim().toLowerCase(), password })
      const destination = new URL(result.url, window.location.origin)
      if (destination.origin !== window.location.origin || !/^\/w\/[a-z0-9-]+(?:\/|$)/.test(destination.pathname)) {
        throw new Error("O Hub retornou um endereço de empresa inválido. Esta sessão não foi redirecionada.")
      }
      window.location.assign(destination.href)
    } catch (e) { setCreateError(e instanceof Error ? e.message : "Não foi possível criar a empresa.") }
    finally { setCreateBusy(false) }
  }
  const downloadExport = async () => {
    setExportBusy(true); setError("")
    try {
      const contents = await api.securityExport()
      const url = URL.createObjectURL(new Blob([JSON.stringify(contents, null, 2)], { type: "application/json" }))
      const link = document.createElement("a")
      link.href = url; link.download = "app-hub-export.json"; link.click()
      URL.revokeObjectURL(url)
    } catch (e) { setError(e instanceof Error ? e.message : "Não foi possível exportar a configuração.") }
    finally { setExportBusy(false) }
  }

  if (loading) return <div role="status" aria-label="Carregando segurança" className="space-y-5"><div className="h-8 w-56 animate-pulse rounded bg-muted" /><div className="h-40 animate-pulse rounded-xl bg-muted" /><div className="h-48 animate-pulse rounded-xl bg-muted" /></div>
  if (error && !info) return <section className="rounded-xl border border-warn/30 bg-warn-soft p-6"><PageHeader title="Segurança" /><p role="alert" className="mt-4">{error}</p><Button className="mt-4" variant="outline" onClick={() => void load()}><RefreshCw className="mr-2 size-4" />Tentar novamente</Button></section>
  if (denied || !info?.can_manage) return <section className="max-w-2xl rounded-xl border border-border bg-card p-7"><PageHeader title="Segurança" /><div className="mt-6 flex gap-4"><LockKeyhole className="size-6 shrink-0 text-muted-foreground" /><div><h2 className="font-heading text-lg">Acesso restrito</h2><p className="mt-2 text-sm text-muted-foreground">Esta área exige a permissão manage_roles. Nenhum dado ou controle de segurança está disponível para este perfil.</p></div></div></section>

  return (
    <>
      <PageHeader title="Segurança" />
      <div className="mb-8 flex items-start gap-3 border-l-2 border-brand pl-4">
        <ShieldCheck className="mt-0.5 size-5 text-brand" />
        <div><h2 className="font-heading text-lg">{workspace?.name ?? "Espaço da empresa"}</h2>
          {workspace && <a href={`${workspace.base_path || ""}/`} className="mt-1 inline-block text-xs font-medium text-brand underline underline-offset-4">Abrir esta empresa</a>}
          <p className="mt-1 max-w-3xl text-sm text-muted-foreground">Controles administrativos e limites técnicos explícitos. Declarações de domínio são informativas; não são regras de saída de rede.</p>
        </div>
      </div>
      {error && <p role="alert" className="mb-5 rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</p>}
      {notice && <p role="status" className="mb-5 rounded-md bg-ok-soft p-3 text-sm">{notice}</p>}

      <section aria-labelledby="limites-heading" className="mb-10 rounded-xl border border-warn/35 bg-warn-soft/50 p-5 sm:p-6">
        <h2 id="limites-heading" className="font-heading text-lg">Limites reais desta instalação</h2>
        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <Limit title="Execução local" value={info.limits.sandboxed ? "Sandbox isolado" : "Não é isolada"} detail="O código roda localmente e não há sandbox." />
          <Limit title="Cofre de segredos" value={info.limits.vault_available ? "Disponível" : "Indisponível"} detail={info.limits.vault_available ? "Cofre configurado para credenciais da empresa." : "Cofre condicionado à configuração do keyring no servidor."} />
          <Limit title="Marketplace externo" value={info.limits.external_marketplace_enabled ? "Habilitado" : "Desabilitado"} detail="Aprovação não autoriza código de marketplace." />
        </div>
        <p className="mt-4 text-sm font-medium">Não insira credenciais em código. Os controles disponíveis não substituem isolamento ou gestão de chaves.</p>
      </section>

      <CredentialVault apps={info.apps} />

      <form onSubmit={saveSettings} className="mb-10 space-y-5 border-b border-border pb-10">
        <div><h2 className="font-heading text-xl">Política da empresa</h2><p className="mt-1 text-sm text-muted-foreground">A retenção se aplica somente a metadados e payloads de execuções finalizadas. Ela só é aplicada quando uma pessoa de gestão aciona o botão abaixo; não há agendamento automático.</p></div>
        <div className="max-w-xs"><Field label="Retenção de execuções finalizadas (dias)" htmlFor="retention"><Input id="retention" type="number" min={1} max={365} required value={settings.retention_days} onChange={(e) => setSettings({ ...settings, retention_days: Number(e.target.value) })} /></Field></div>
        <div className="space-y-3">
          <Toggle label="Exigir aprovação de aplicativo" detail="A aprovação vale para o manifesto e a versão atual do código. Mudanças no código exigem nova revisão; aprovação não cria sandbox." checked={settings.require_app_approval} onChange={(v) => setSettings({ ...settings, require_app_approval: v })} />
          <Toggle label="Bloquear segredos embutidos" detail="Bloqueia a política para segredos encontrados no código; não é um cofre." checked={settings.block_embedded_secrets} onChange={(v) => setSettings({ ...settings, block_embedded_secrets: v })} />
          <Toggle label="Permitir execução local" detail="Ao salvar desativado, novos pedidos serão bloqueados e jobs ainda não reivindicados serão cancelados. Scripts já em execução ou em agentes offline não serão interrompidos." checked={settings.local_execution_enabled} onChange={(v) => setSettings({ ...settings, local_execution_enabled: v })} />
        </div>
        <div className="flex flex-wrap gap-2"><Button type="submit" disabled={saving}>{saving ? "Salvando…" : "Salvar política"}</Button><Button type="button" variant="outline" onClick={() => void load()}>Atualizar dados</Button></div>
      </form>

      <section className="mb-10">
        <h2 className="font-heading text-xl">Perfis de segurança dos aplicativos</h2>
        <p className="mb-5 mt-1 max-w-3xl text-sm text-muted-foreground">Classificação, finalidade e domínios declarados são metadados de governança. Domínios externos não são bloqueados nem autorizados por esta configuração.</p>
        {info.apps.length === 0 ? <p className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">Nenhum aplicativo registrado neste espaço.</p> : <div className="space-y-4">{info.apps.map((app) => <AppProfile key={app.id} app={app} onSave={saveApp} />)}</div>}
      </section>

      <section className="mb-10 grid gap-8 border-y border-border py-8 lg:grid-cols-[1fr_1.2fr]">
        <div>
          <h2 className="font-heading text-xl">Limpar execuções antigas</h2>
          <p className="mt-2 text-sm text-muted-foreground">Remove apenas metadados e payloads de execuções finalizadas conforme a retenção configurada.</p>
          <p className="mt-2 flex gap-2 text-sm"><AlertTriangle className="size-4 shrink-0 text-warn" />Não apaga arquivos do Windows nem backups do banco de dados.</p>
          <Button variant="destructive" className="mt-4" disabled={purging} onClick={() => void purge()}>{purging ? "Aplicando retenção…" : "Aplicar retenção agora"}</Button>
        </div>
        <div>
          <h2 className="font-heading text-xl">Registro administrativo</h2>
          <p className="mt-1 text-sm text-muted-foreground">Eventos mais recentes, até 100. O registro não contém valores de credenciais.</p>
          {!audit?.events.length ? <p className="mt-4 text-sm text-muted-foreground">Nenhum evento disponível.</p> : <ol className="mt-4 max-h-72 divide-y overflow-auto">{audit.events.map((event) => <li key={event.id} className="grid grid-cols-[1fr_auto] gap-2 py-3 text-sm"><span className="font-medium">{event.action}</span><time className="text-xs text-muted-foreground">{new Date(event.created_at).toLocaleString("pt-BR")}</time><span className="col-span-2 text-xs text-muted-foreground">{event.route} · HTTP {event.status} · {event.actor_id == null ? "Sistema" : `Pessoa ${event.actor_id}`}</span></li>)}</ol>}
        </div>
      </section>

      <section className="mb-10 flex flex-wrap items-center justify-between gap-4">
        <div><h2 className="font-heading text-lg">Exportar configuração</h2><p className="mt-1 text-sm text-muted-foreground">Inclui configuração e metadados da empresa; não inclui valores de credenciais, código ou entradas.</p></div>
        <Button type="button" variant="outline" disabled={exportBusy} onClick={() => void downloadExport()}><Download className="mr-2 size-4" />{exportBusy ? "Preparando…" : "Baixar exportação JSON"}</Button>
      </section>

      <section className="rounded-xl border border-border bg-card p-5 sm:p-6">
        <h2 className="font-heading text-xl">Criar outro espaço de empresa</h2>
        <p className="mt-2 max-w-3xl text-sm text-muted-foreground">Cria uma conta independente com as credenciais atuais. O nome, e-mail e hash da senha do administrador inicial são reutilizados; na primeira visita à nova empresa, entre com essas credenciais. Depois, cada empresa administra usuários, sessões e dados separadamente: não há SSO entre empresas. A sessão da empresa atual continua preservada em seu endereço próprio. Não existe diretório ou listagem de outras empresas.</p>
        {!creating ? <Button variant="outline" className="mt-4" onClick={() => setCreating(true)}>Criar espaço de empresa</Button> : <form onSubmit={createCompany} className="mt-5 grid max-w-xl gap-4">
          <Field label="Nome da empresa" htmlFor="company-name"><Input id="company-name" value={companyName} onChange={(e) => setCompanyName(e.target.value)} required maxLength={120} /></Field>
          <Field label="Endereço do espaço" htmlFor="company-slug" hint="Letras minúsculas, números e hífens."><Input id="company-slug" value={companySlug} onChange={(e) => setCompanySlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))} pattern="[a-z0-9-]+" required /></Field>
          <Field label="Confirme sua senha atual" htmlFor="company-password"><Input id="company-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" required /></Field>
          {createError && <p role="alert" className="text-sm text-destructive">{createError}</p>}
          <div className="flex flex-wrap gap-2"><Button type="submit" disabled={createBusy}>{createBusy ? "Criando…" : "Criar e abrir empresa"}</Button><Button type="button" variant="ghost" onClick={() => { setCreating(false); setCreateError("") }}>Cancelar</Button></div>
        </form>}
      </section>
    </>
  )
}

function Limit({ title, value, detail }: { title: string; value: string; detail: string }) {
  return <div className="rounded-lg border border-border/70 bg-background/70 p-4"><p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{title}</p><p className="mt-2 font-heading">{value}</p><p className="mt-1 text-xs text-muted-foreground">{detail}</p></div>
}

function Toggle({ label, detail, checked, onChange }: { label: string; detail: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border p-3"><input type="checkbox" className="mt-1 size-4 accent-[var(--brand)]" checked={checked} onChange={(e) => onChange(e.target.checked)} /><span><span className="text-sm font-medium">{label}</span><span className="mt-0.5 block text-xs text-muted-foreground">{detail}</span></span></label>
}

function AppProfile({ app, onSave }: { app: SecurityApp; onSave: (app: SecurityApp, form: HTMLFormElement) => void }) {
  return <form onSubmit={(event) => { event.preventDefault(); void onSave(app, event.currentTarget) }} className="grid gap-4 rounded-xl border border-border bg-card p-4 sm:p-5 lg:grid-cols-[1fr_1fr]">
    <div className="lg:col-span-2 flex flex-wrap items-baseline justify-between gap-2"><h3 className="font-heading text-lg">{app.name}</h3><span className="font-mono text-xs text-muted-foreground">ID {app.id}</span></div>
    <label className="text-sm font-medium">Classificação<select name="classification" defaultValue={["internal", "confidential", "restricted"].includes(app.classification) ? app.classification : "internal"} className={inputClass}><option value="internal">Interno</option><option value="confidential">Confidencial</option><option value="restricted">Restrito</option></select></label>
    <label className="text-sm font-medium">Finalidade<textarea name="purpose" defaultValue={app.purpose} maxLength={1000} rows={3} className={inputClass} /></label>
    <label className="text-sm font-medium lg:col-span-2">Domínios externos declarados<textarea name="domains" defaultValue={app.external_domains.join("\n")} rows={2} placeholder="um domínio por linha" className={inputClass} /><span className="mt-1 block text-xs font-normal text-muted-foreground">Informativo. Não é uma lista de permissão nem regra de firewall.</span></label>
    <div className="flex flex-wrap gap-4 lg:col-span-2">
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" name="approved" defaultChecked={app.approved} className="size-4 accent-[var(--brand)]" />Aprovado como código interno confiável</label>
      <label className="flex items-center gap-2 text-sm text-destructive"><input type="checkbox" name="suspended" defaultChecked={app.suspended} className="size-4 accent-[var(--brand)]" />Suspenso — impedir execução</label>
    </div>
    <div className="lg:col-span-2"><Button type="submit" variant="outline">Salvar perfil</Button></div>
  </form>
}
