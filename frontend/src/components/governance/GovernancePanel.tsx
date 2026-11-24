import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { api, type CompanyMCP, type GovernanceEntry, type GovernanceSettings, type VersionRule } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { RULES } from "./RuleSection"
import { reasonLabel } from "./RequestCard"

const EVENT: Record<string, string> = {
  settings: "mudou as regras de aprovação", audience: "mudou quem usa", rule: "mudou a regra de versões",
  mcp: "cancelou uma chave do MCP", area: "mudou a área",
}

function entryText(e: GovernanceEntry): string {
  if (e.type === "request" && e.request) {
    const r = e.request
    const who = r.steps.filter((s) => s.decision).map((s) => `${s.decision === "approved" ? "aprovado" : "recusado"} por ${s.decided_by}`).join(", ")
    return `${r.item_name} · ${reasonLabel(r)}${r.requested_by ? ` · pedido por ${r.requested_by}` : ""}${who ? ` · ${who}` : r.status === "auto" ? " · foi ao ar com aviso" : r.status === "open" ? " · esperando" : ""}`
  }
  const ev = e.event
  return ev ? `${ev.actor || "Alguém"} ${EVENT[ev.kind] ?? ev.kind}` : ""
}

/** Admin → Governança: how the company approves, MCP keys and the history (ADR 046). */
export function GovernancePanel() {
  const [s, setS] = useState<GovernanceSettings | null>(null)
  const [keys, setKeys] = useState<CompanyMCP[] | null>(null)
  const [history, setHistory] = useState<GovernanceEntry[] | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let live = true
    api.governanceSettings().then((x) => live && setS(x), () => {})
    api.companyMCP().then((x) => live && setKeys(x), () => live && setKeys([]))
    api.governanceHistory().then((x) => live && setHistory(x), () => live && setHistory([]))
    return () => { live = false }
  }, [])
  if (!s) return null

  const save = async (next: GovernanceSettings) => {
    setBusy(true)
    try {
      setS(await api.saveGovernanceSettings(next))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy(false)
    }
  }
  const revoke = async (k: CompanyMCP) => {
    if (!window.confirm(`Cancelar a chave ${k.name} de ${k.person_name}? O Claude dessa pessoa perde o acesso ao Ziv na hora.`)) return
    try {
      await api.revokeCompanyMCP(k.id)
      setKeys((l) => l?.map((x) => (x.id === k.id ? { ...x, revoked: true } : x)) ?? l)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível cancelar.")
    }
  }
  const radio = (name: string, checked: boolean, label: string, onPick: () => void) => (
    <label className="flex items-center gap-2">
      <input type="radio" name={name} className="accent-foreground" checked={checked} disabled={busy} onChange={onPick} />
      {label}
    </label>
  )
  const activeKeys = keys?.filter((k) => !k.revoked) ?? []

  return (
    <section aria-labelledby="governanca" className="border-t border-border pt-8">
      <h2 id="governanca" className="font-heading text-xl">Governança</h2>
      <div className="mt-4 flex flex-col gap-5 rounded-xl border border-border bg-card p-4 text-sm">
        <p>O líder da área sempre aprova.</p>
        <label className="flex items-center gap-2">
          <input type="checkbox" className="size-4 accent-foreground" checked={s.it_approval} disabled={busy} onChange={(e) => void save({ ...s, it_approval: e.target.checked })} />
          A TI também aprova (quem tem "Aprovar pela TI" no cargo)
        </label>
        <fieldset className="flex flex-col gap-1.5">
          <legend className="mb-1.5 font-medium">Versão nova precisa de aprovação:</legend>
          {RULES.map((r) => <div key={r.key}>{radio("gov-rule", s.version_rule === r.key, r.label, () => void save({ ...s, version_rule: r.key as VersionRule }))}</div>)}
        </fieldset>
        <fieldset className="flex flex-col gap-1.5">
          <legend className="mb-1.5 font-medium">Por onde o código pode chegar:</legend>
          {radio("gov-channel", s.code_channel === "any", "Qualquer caminho", () => void save({ ...s, code_channel: "any" }))}
          {radio("gov-channel", s.code_channel === "mcp", "Só pelo Claude", () => void save({ ...s, code_channel: "mcp" }))}
          <p className="text-xs text-muted-foreground">Com "Só pelo Claude", apps e artifacts só chegam pelo Claude conectado ao Ziv; o que já está no ar continua rodando.</p>
        </fieldset>
        <label className="flex items-center gap-2">
          <input type="checkbox" className="size-4 accent-foreground" checked={s.mcp_restricted} disabled={busy} onChange={(e) => void save({ ...s, mcp_restricted: e.target.checked })} />
          Só cargos com "Conectar o Claude" podem ter chave do MCP
        </label>
      </div>

      <h3 className="mt-8 font-heading text-base">Chaves do MCP</h3>
      {keys === null ? <p className="mt-2 text-sm text-muted-foreground">Carregando…</p> : activeKeys.length === 0 ? (
        <p className="mt-2 text-sm text-muted-foreground">Ninguém conectou o Claude ainda.</p>
      ) : (
        <ul className="mt-2 divide-y divide-border rounded-xl border border-border bg-card text-sm">
          {activeKeys.map((k) => (
            <li key={k.id} className="flex flex-wrap items-center justify-between gap-3 px-4 py-2.5">
              <span>
                <span className="font-medium">{k.person_name}</span> · {k.kind === "oauth" ? "claude.ai" : k.name}
                <span className="block text-xs text-muted-foreground">Criada {formatWhen(k.created_at)}{k.last_used_at ? ` · usada ${formatWhen(k.last_used_at)}` : " · nunca usada"}{k.expires_at && ` · vale até ${new Date(k.expires_at).toLocaleDateString("pt-BR")}`}</span>
              </span>
              <Button size="sm" variant="outline" aria-label={`Cancelar a chave ${k.name} de ${k.person_name}`} onClick={() => void revoke(k)}>Cancelar</Button>
            </li>
          ))}
        </ul>
      )}

      <h3 className="mt-8 font-heading text-base">Histórico de governança</h3>
      {history === null ? <p className="mt-2 text-sm text-muted-foreground">Carregando…</p> : history.length === 0 ? (
        <p className="mt-2 text-sm text-muted-foreground">Nada registrado ainda.</p>
      ) : (
        <ol className="mt-2 divide-y divide-border rounded-xl border border-border bg-card text-sm">
          {history.slice(0, 50).map((e, i) => (
            <li key={`${e.type}-${e.request?.id ?? e.event?.id ?? i}`} className="flex justify-between gap-3 px-4 py-2.5">
              <span>{entryText(e)}</span>
              <time className="shrink-0 text-xs text-muted-foreground">{formatWhen(e.at)}</time>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
