import type { Person } from "./api"

export const PERMISSIONS: { key: string; label: string; hint: string }[] = [
  { key: "register_apps", label: "Registrar apps", hint: "Cadastra apps novos no catálogo" },
  { key: "edit_any_app", label: "Editar qualquer app", hint: "Muda a ficha de todos os apps" },
  { key: "restore_any_app", label: "Voltar versões de qualquer app", hint: "Desfaz mudanças no código de todos os apps" },
  { key: "see_tech_any_app", label: "Ver a parte técnica de qualquer app", hint: "Código, chaves, bibliotecas e histórico" },
  { key: "see_all_runs", label: "Ver execuções de todos", hint: "Quem rodou o quê, em todos os apps" },
  { key: "see_all_apps", label: "Ver todos os apps", hint: "Inclusive os restritos" },
  { key: "manage_people", label: "Cuidar das pessoas", hint: "Cadastra pessoas e escolhe o cargo delas" },
  { key: "manage_areas", label: "Cuidar das áreas", hint: "Cria áreas e escolhe os líderes" },
  { key: "manage_roles", label: "Cuidar dos cargos", hint: "Cria cargos e escolhe o que cada um pode" },
  { key: "manage_server", label: "Cuidar do servidor", hint: "Vê gastos e uso do servidor, muda limites e para qualquer app compartilhado" },
]

export const GRANTS: { key: string; label: string }[] = [
  { key: "edit", label: "Editar a ficha" },
  { key: "restore", label: "Voltar versões" },
  { key: "tech", label: "Ver a parte técnica" },
  { key: "runs", label: "Ver execuções de todos" },
]

export const has = (p: Person | null | undefined, perm: string) => !!p?.permissions.includes(perm)
export const canRegister = (p: Person | null | undefined) => has(p, "register_apps") || (p?.leads.length ?? 0) > 0
export const canSeeTeam = (p: Person | null | undefined) =>
  has(p, "manage_people") || has(p, "manage_areas") || has(p, "manage_roles") || (p?.leads.length ?? 0) > 0
export const canSeeServer = (p: Person | null | undefined) => has(p, "manage_server")
export const permLabel = (key: string) => PERMISSIONS.find((x) => x.key === key)?.label ?? GRANTS.find((x) => x.key === key)?.label ?? key
