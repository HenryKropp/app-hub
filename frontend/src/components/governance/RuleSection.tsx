import { useState } from "react"
import { toast } from "sonner"
import { NativeSelect } from "@/components/bits"
import { Section } from "@/components/ficha/Part"
import { api, type ItemGovernance, type VersionRule } from "@/lib/api"

export const RULES: { key: VersionRule; label: string; hint: string }[] = [
  { key: "every", label: "Toda versão", hint: "Cada mudança no código espera aprovação." },
  { key: "risk", label: "Só quando muda o risco", hint: "Espera só se a versão passa a acessar algo novo (site, pasta, chave)." },
  { key: "first", label: "Só a primeira", hint: "Depois de aprovado uma vez, as versões vão direto ao ar." },
]
const rank = (r: VersionRule) => RULES.findIndex((x) => x.key === r) // lower = stricter
export const ruleLabel = (r: VersionRule) => RULES.find((x) => x.key === r)?.label ?? r

/**
 * Aprovação de versões (Configurações of an app or artifact): the company's
 * rule and this item's own. Who cares for it can only make it stricter; the
 * admin (can_loosen) may also make it lighter (ADR 046).
 */
export function RuleSection({ kind, id, gov, onChanged }: { kind: "app" | "artifact"; id: number; gov: ItemGovernance; onChanged: (g: ItemGovernance) => void }) {
  const [busy, setBusy] = useState(false)
  const company = gov.rule_company
  const options = RULES.filter((r) => r.key !== company && (gov.can_loosen || rank(r.key) < rank(company)))

  const save = async (value: string) => {
    setBusy(true)
    try {
      const rule = value === "" ? null : (value as VersionRule)
      onChanged(await (kind === "app" ? api.setAppRule(id, rule) : api.setArtifactRule(id, rule)))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Section title="Aprovação de versões" id={`regra-${kind}-${id}`}>
      <div className="text-sm">
        <p className="text-muted-foreground">
          O líder da área aprova{gov.it_on ? " e a TI também" : ""}. Padrão da empresa: {ruleLabel(company).toLowerCase()}.
        </p>
        {gov.can_change ? (
          <div className="mt-3 flex flex-col gap-1.5">
            <label htmlFor={`regra-sel-${kind}-${id}`} className="font-medium">Versão nova precisa de aprovação</label>
            <NativeSelect id={`regra-sel-${kind}-${id}`} className="max-w-xs" value={gov.rule_item} disabled={busy} onChange={(e) => void save(e.target.value)}>
              <option value="">Igual à empresa ({ruleLabel(company).toLowerCase()})</option>
              {options.map((r) => <option key={r.key} value={r.key}>{r.label}</option>)}
            </NativeSelect>
            <p className="text-xs text-muted-foreground">{RULES.find((r) => r.key === gov.rule_effective)?.hint}</p>
          </div>
        ) : (
          <p className="mt-3">Versão nova precisa de aprovação: <span className="font-medium">{ruleLabel(gov.rule_effective)}</span>{gov.rule_item ? " (regra deste item)" : ""}.</p>
        )}
      </div>
    </Section>
  )
}
