import type { Input, Kind, Role } from "./api"

export const KINDS: { value: Kind; label: string; short: string; color: string }[] = [
  { value: "script", label: "Script: roda e entrega um resultado", short: "Script", color: "var(--kind-script)" },
  { value: "web", label: "App com tela no navegador", short: "Navegador", color: "var(--kind-web)" },
  { value: "desktop", label: "App com janela própria", short: "Janela", color: "var(--kind-desktop)" },
  { value: "scheduled", label: "Tarefa recorrente", short: "Recorrente", color: "var(--kind-scheduled)" },
  { value: "executable", label: "Executável pronto (.exe)", short: "Executável", color: "var(--kind-executable)" },
  { value: "reference", label: "Documento ou atalho (não roda)", short: "Documento", color: "var(--kind-reference)" },
  { value: "system", label: "Sistema complexo (só registro)", short: "Sistema", color: "var(--kind-system)" },
]

export const kindInfo = (k: Kind) => KINDS.find((x) => x.value === k) ?? KINDS[0]

export const RUNNABLE: Kind[] = ["script", "web", "desktop", "scheduled", "executable"]

export const INPUT_TYPES: { value: Input["type"]; label: string }[] = [
  { value: "file", label: "Arquivo" },
  { value: "folder", label: "Pasta" },
  { value: "text", label: "Texto" },
  { value: "number", label: "Número" },
  { value: "date", label: "Data" },
  { value: "choice", label: "Lista de opções" },
]

export const INPUT_SOURCES: Record<Input["source"], string> = {
  fixed_file: "arquivo fixo na pasta",
  dialog: "janela de seleção",
  prompt: "pergunta no terminal",
  argument: "parâmetro",
  window: "campo na janela do app",
}

export const ROLES: { value: Role; label: string; hint: string }[] = [
  { value: "admin", label: "Administrador", hint: "Cadastra pessoas e vê tudo" },
  { value: "criador", label: "Criador", hint: "Registra e mantém apps" },
  { value: "usuario", label: "Usuário", hint: "Encontra e usa apps" },
]

export const roleLabel = (r: Role) => ROLES.find((x) => x.value === r)?.label ?? r

export const AREAS = ["Investimentos", "Operações", "Relações com Investidores", "Research", "Compliance", "Administrativo"]
