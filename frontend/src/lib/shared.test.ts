import { describe, expect, it } from "vitest"
import { SHARED_STATUS, claudeTextFor, formatCents, sharedLine } from "./shared"
import type { SharedState } from "./api"

const base: SharedState = { status: "running", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30,
  memory_mb: 512, cpus: 0.5, wants_db: true, url: "http://notas.localhost:8080", acks: [], has_previous: false }

describe("sharedLine", () => {
  it("says it is on and which version", () => {
    expect(sharedLine(base, 4)).toBe("No ar · versão 4")
  })
  it("explains sleeping in plain words", () => {
    expect(sharedLine({ ...base, status: "sleeping" }, 4)).toBe("Descansando · acorda em segundos quando alguém abre")
  })
  it("names who stopped it", () => {
    expect(sharedLine({ ...base, status: "stopped", stopped_by: "Henry" })).toBe("Parado por Henry")
  })
  it("says when it fell", () => {
    expect(sharedLine({ ...base, status: "crashed" })).toBe("Caiu e não voltou sozinho")
  })
  it("has a label for every status", () => {
    for (const s of ["draft", "starting", "running", "sleeping", "stopped", "crashed"] as const) expect(SHARED_STATUS[s].label).not.toBe("")
  })
})

it("formats money in reais", () => {
  expect(formatCents(123456)).toBe("R$ 1.234,56")
  expect(formatCents(0)).toBe("R$ 0,00")
})

it("joins the Claude texts of what is missing", () => {
  const t = claudeTextFor([
    { key: "port", status: "missing", title: "Porta", detail: "", claude_text: "Faça o app ler a porta da variável PORT.", acked: false },
    { key: "recipe", status: "ok", title: "Receita", detail: "", claude_text: "", acked: false },
    { key: "file_data", status: "warn", title: "Dados", detail: "", claude_text: "Guarde os dados em APPHUB_DATA_DIR.", acked: false },
  ])
  expect(t).toContain("Faça o app ler a porta")
  expect(t).toContain("APPHUB_DATA_DIR")
  expect(t.startsWith("Estou preparando este app para rodar compartilhado no App Hub.")).toBe(true)
})
