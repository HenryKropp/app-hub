import type { Computer, Manifest, Person, Run, ScanItem } from "@/lib/api"

export const henry: Person = { id: 1, name: "Henry", email: "henry@alis.com", role: "admin" }
export const ana: Person = { id: 2, name: "Ana", email: "ana@alis.com", role: "criador" }

export function manifest(over: Partial<Manifest> = {}): Manifest {
  return {
    manifest_version: 1,
    id: "rentabilidade",
    name: "Rentabilidade fundos",
    summary: "",
    kind: "script",
    language: "python",
    entrypoint: "rentabilidade.py",
    run: { command: 'python "rentabilidade.py"', url: "" },
    runtime: { language: "python", dependencies_file: "", dependencies: ["pandas"] },
    inputs: [{ id: "cnpjs", label: "Arquivo cnpjs.csv", type: "file", source: "fixed_file", path: "cnpjs.csv", required: true }],
    outputs: [{ label: "Planilha gerada", pattern: "rentabilidade_*.xlsx" }],
    secrets: null,
    schedule: "",
    area: "",
    owner: null,
    backup: null,
    tags: null,
    notes: ["O app lê um arquivo com nome fixo na própria pasta."],
    ...over,
  }
}

export function scanItem(over: Partial<ScanItem> = {}): ScanItem {
  return {
    folder_path: "/mnt/c/apps/Rentabilidade",
    folder_name: "Rentabilidade",
    folder_display: "C:\\apps\\Rentabilidade",
    manifest: manifest(),
    findings: ["Linguagem principal: Python."],
    questions: [{ field: "inputs", text: "O app lê o arquivo cnpjs.csv da própria pasta. Ele muda a cada uso?" }],
    entrypoint_options: ["rentabilidade.py"],
    secrets: null,
    file_count: 3,
    truncated: false,
    has_manifest: false,
    suggest_skip: false,
    registered_app_id: 0,
    similar: [],
    ...over,
  }
}

export function computer(over: Partial<Computer> = {}): Computer {
  return { id: 1, name: "NOTE-HENRY", agent_version: "0.1.0", last_seen_at: "2026-09-30T14:00:00Z", connected: true, outdated: false, update_state: "", ...over }
}

export function run(over: Partial<Run> = {}): Run {
  return {
    id: 7, app_id: 3, app_name: "Eq. Research Files Prep", person_name: "Henry", computer_name: "NOTE-HENRY",
    output_folder: "C:\\apps\\Files Prep", status: "done", exit_code: 0, error_message: "", app_version: "2026-08-13 17:54 · 75,6 MB", version_number: null,
    created_at: "2026-09-30T14:10:00Z", started_at: "2026-09-30T14:10:05Z", finished_at: "2026-09-30T14:22:05Z",
    groups: [
      { index: 0, label: "Documentos da CVM", files: [{ id: 1, path: "PETROBRAS/FRE/FRE_2025.pdf", size: 2048, modified: "2026-09-30T14:20:00Z", change: "new" }] },
      { index: 1, label: "Data Pack", files: [{ id: 2, path: "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", size: 900, modified: "2026-09-30T14:21:00Z", change: "new" }] },
      { index: 2, label: "FRE cortado", files: [] },
      { index: -1, label: "Outros arquivos", files: [{ id: 3, path: "log.txt", size: 10, modified: "2026-09-30T14:21:00Z", change: "changed" }] },
    ],
    ...over,
  }
}
