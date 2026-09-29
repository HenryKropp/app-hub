import type { Manifest, Person, ScanItem } from "@/lib/api"

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
